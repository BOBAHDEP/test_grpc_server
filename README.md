# test_grpc_server
Example of gRPC using postgres

## Usage

First, start a postgres container:

```bash
$ docker run --rm -d --name postgres -p 5432:5432 -e POSTGRES_USER=postgres -e POSTGRES_PASSWORD=mypass -e POSTGRES_DB=postgres postgres:13
d1a2eb0fb44da9c3488184f5296da28d1c7f88bd32bd4ec81fc254f006886b03
```

Start the server:

```bash
$ POSTGRES_URL=postgresql://postgres:mypass@localhost:5432/postgres?sslmode=disable go run main.go
```

Navigate to http://0.0.0.0:8080 to see the auto-generated web UI for gRPC
