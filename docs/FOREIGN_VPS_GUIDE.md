# Foreign VPS notes

The production Compose profile is provider-neutral. For a VPS outside Iran:

- Select a region with acceptable latency for the hotel and its guests.
- Keep PostgreSQL private; the production profile does not publish port 5432.
- Point DNS to the VPS before issuing the TLS certificate.
- Allow only SSH, HTTP, and HTTPS at the network firewall.
- Use an SSH key, disable password-based root login, and install automatic security updates.
- Confirm the provider's backup retention and data-residency terms because guest identity and operational records are sensitive.
- Keep third-party AI, SMS, payment, and object-storage providers behind configuration boundaries; those integrations are not part of M1.

Follow `docs/DEPLOYMENT_GUIDE.md` for the concrete Compose commands. No provider credentials, IP addresses, or private keys should be committed to this repository.
