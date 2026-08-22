package operations

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/TablazOrg/HotelMate/backend/internal/database"
)

const recoveryDrillSchemaVersion = "hotelmate.recovery-drill/v1"

type recoveryDrillEvidence struct {
	SchemaVersion          string    `json:"schemaVersion"`
	Status                 string    `json:"status"`
	Environment            string    `json:"environment"`
	Operator               string    `json:"operator"`
	RequestedRecoveryPoint time.Time `json:"requestedRecoveryPoint"`
	SourceRecoverySet      string    `json:"sourceRecoverySet"`
	SourceCreatedAt        time.Time `json:"sourceCreatedAt"`
	ReleaseVersion         string    `json:"releaseVersion"`
	StartedAt              time.Time `json:"startedAt"`
	FinishedAt             time.Time `json:"finishedAt"`
	RPOSeconds             int64     `json:"rpoSeconds"`
	RTOSeconds             int64     `json:"rtoSeconds"`
	RPOObjectiveSeconds    int64     `json:"rpoObjectiveSeconds,omitempty"`
	RTOObjectiveSeconds    int64     `json:"rtoObjectiveSeconds,omitempty"`
	Verification           any       `json:"verification,omitempty"`
	Restore                any       `json:"restore,omitempty"`
	Migrations             any       `json:"migrations,omitempty"`
	Smoke                  any       `json:"smoke,omitempty"`
	Acceptance             any       `json:"acceptance,omitempty"`
	Error                  string    `json:"error,omitempty"`
}

func (a *App) recoveryDrill(ctx context.Context, config resolvedConfig) (any, error) {
	if config.Environment == "production" {
		return nil, failure(ExitPrecondition, "recovery drills must target an isolated non-production environment", nil)
	}
	if !config.RecoveryDrill {
		return nil, failure(ExitPrecondition, "set HOTELMATE_ISOLATED_RECOVERY_DRILL=true in the dedicated drill configuration", nil)
	}
	requestedPoint := a.Now().UTC()
	if config.RecoveryPoint != "" {
		parsed, err := time.Parse(time.RFC3339, config.RecoveryPoint)
		if err != nil {
			return nil, failure(ExitInvalid, "HOTELMATE_RECOVERY_POINT must use RFC3339", err)
		}
		requestedPoint = parsed.UTC()
	}
	manifest, err := recoverySetAt(config, requestedPoint)
	if err != nil {
		return nil, err
	}
	if manifest.CreatedAt.After(requestedPoint) {
		return nil, failure(ExitInvalid, "selected recovery set is newer than the requested recovery point", nil)
	}
	started := a.Now().UTC()
	evidence := recoveryDrillEvidence{
		SchemaVersion: recoveryDrillSchemaVersion, Status: "started", Environment: config.Environment,
		Operator: config.DrillOperator, RequestedRecoveryPoint: requestedPoint,
		SourceRecoverySet: manifest.ID, SourceCreatedAt: manifest.CreatedAt, ReleaseVersion: manifest.ReleaseVersion,
		StartedAt: started, RPOSeconds: int64(requestedPoint.Sub(manifest.CreatedAt).Seconds()),
		RPOObjectiveSeconds: int64(config.DrillMaxRPO.Seconds()), RTOObjectiveSeconds: int64(config.DrillMaxRTO.Seconds()),
	}
	finish := func(status string, commandErr error) (any, error) {
		evidence.Status = status
		evidence.FinishedAt = a.Now().UTC()
		evidence.RTOSeconds = int64(evidence.FinishedAt.Sub(started).Seconds())
		if commandErr != nil {
			evidence.Error = commandErr.Error()
		}
		path := filepath.Join(config.EvidenceDir, config.Environment, "recovery-drills", started.Format("20060102T150405.000000000Z")+"-"+status+".json")
		if writeErr := writeJSONAtomic(path, evidence, 0o600); writeErr != nil {
			return evidence, failure(ExitVerification, "record recovery-drill evidence failed", writeErr)
		}
		return evidence, commandErr
	}

	releaseLock, err := acquireEnvironmentLock(config.EvidenceDir, config.Environment)
	if err != nil {
		return finish("failed", err)
	}
	defer releaseLock()

	verification, err := a.verifyBackup(ctx, config, manifest)
	evidence.Verification = verification
	if err != nil {
		return finish("verification_failed", err)
	}
	restore, err := a.restoreBackup(ctx, config, manifest)
	evidence.Restore = restore
	if err != nil {
		return finish("restore_failed", err)
	}
	databaseContext, cancel := context.WithTimeout(ctx, 2*time.Minute)
	db, err := database.Open(databaseContext, config.DatabaseURL)
	if err == nil {
		err = database.Migrate(db)
	}
	var migrations any
	if err == nil {
		migrations, err = database.MigrationStatuses(db)
	}
	if db != nil {
		_ = database.Close(db)
	}
	cancel()
	evidence.Migrations = migrations
	if err != nil {
		return finish("migration_failed", failure(ExitCommand, "recovery-drill migration failed", err))
	}
	smoke, err := a.waitForSmoke(ctx, config, 60*time.Second)
	evidence.Smoke = smoke
	if err != nil {
		return finish("smoke_failed", err)
	}
	acceptance, err := a.acceptance(ctx, config)
	evidence.Acceptance = acceptance
	if err != nil {
		return finish("acceptance_failed", err)
	}
	actualRTO := a.Now().UTC().Sub(started)
	if config.DrillMaxRPO > 0 && time.Duration(evidence.RPOSeconds)*time.Second > config.DrillMaxRPO {
		return finish("objective_failed", failure(ExitVerification, fmt.Sprintf("measured RPO %ds exceeds objective %ds", evidence.RPOSeconds, int64(config.DrillMaxRPO.Seconds())), nil))
	}
	if config.DrillMaxRTO > 0 && actualRTO > config.DrillMaxRTO {
		return finish("objective_failed", failure(ExitVerification, fmt.Sprintf("measured RTO %ds exceeds objective %ds", int64(actualRTO.Seconds()), int64(config.DrillMaxRTO.Seconds())), nil))
	}
	return finish("succeeded", nil)
}

func recoverySetAt(config resolvedConfig, requestedPoint time.Time) (recoverySet, error) {
	if config.ManifestFile != "" {
		return resolveRecoverySet(config)
	}
	data, err := listBackups(config.BackupDir)
	if err != nil {
		return recoverySet{}, err
	}
	sets := data.(map[string]any)["recoverySets"].([]recoverySet)
	for _, set := range sets {
		if !set.CreatedAt.After(requestedPoint) {
			return set, nil
		}
	}
	return recoverySet{}, failure(ExitPrecondition, "no recovery set exists at or before the requested recovery point", nil)
}
