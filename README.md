# pipery-release-bot

Small HTTP service that executes a configured GitHub release plan with a GitHub App installation token.

## Configuration

Set `PIPERY_RELEASE_CONFIG` to a JSON file:

```json
{
  "listen_addr": ":8080",
  "target": {
    "owner": "pipery-dev",
    "repo": "example",
    "base_ref": "main",
    "version": "v1.2.3",
    "release_notes_path": "CHANGELOG.md"
  },
  "branch_patterns": [
    {
      "pattern": "release/{version}",
      "create_tag": true,
      "tag_name": "{version}",
      "create_release": true
    }
  ],
  "installations": {
    "default": {
      "app_id": 12345,
      "installation_id": 67890,
      "private_key_file": "/run/secrets/github-app.pem"
    }
  }
}
```

Private keys are loaded from `private_key_file` or `private_key_env`; do not put key material in source control.

Set `api_token` in the config or `PIPERY_RELEASE_API_TOKEN` in the environment to require `Authorization: Bearer <token>` for the release execution API.

## API

```sh
curl http://localhost:8080/healthz
```

```sh
curl -X POST http://localhost:8080/v1/release-plans/execute \
  -H 'Content-Type: application/json' \
  -d '{
    "installation_key": "default",
    "version": "v1.2.3",
    "base_ref": "main"
  }'
```

The request can override `owner`, `repo`, `version`, `base_ref`, and `release_notes_path`; unset values use configuration defaults.
