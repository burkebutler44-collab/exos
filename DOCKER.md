# Relay Docker Stack

Run the customer app, admin app, API, Postgres, and NATS with:

```sh
docker compose up --build
```

Then open:

- Customer app: http://localhost:5173
- Admin app: http://localhost:5174
- API health: http://localhost:8080/healthz
- gRPC API: localhost:9090
- NATS: localhost:4222
- NATS monitoring: http://localhost:8222
- Postgres: localhost:5432

The API container runs Goose migrations before starting. Local Postgres data is stored in the `relay-postgres-data` Docker volume.

To reset the database:

```sh
docker compose down -v
docker compose up --build
```
