# Mock server

The mock server exposes the same API shape as the app server for frontend design checks.
Connect the frontend SSE client to `/api/events?demo=1` to receive:

- `init` with generated channel topology.
- `trigger` with random `mov` channel movement traffic and occasional `msg` impulses.
- `sync` score deltas derived from the generated activity.

## Activity tuning

Use these environment variables to adjust the generated traffic:

- `MOCK_VERTEX_COUNT`: number of generated channels. Defaults to `129`.
- `MOCK_ACTIVITY_INTERVAL`: interval between generated trigger events. Defaults to `350ms`.
- `MOCK_ACTIVITY_USERS`: number of simulated users moving between channels. Defaults to `80`.
- `MOCK_MESSAGE_EVERY`: emit one `msg` trigger every N events; all other events are `mov`. Defaults to `5`.

For example, `MOCK_ACTIVITY_INTERVAL=100ms MOCK_ACTIVITY_USERS=200 MOCK_MESSAGE_EVERY=10 go run .` creates heavier movement traffic for animation stress checks.
