# Todo App with Aserto Authorization

This application demonstrates how to use Aserto's Go SDK to add an authorization layer to a simple todo app.

## Set up an `.env` file
Create the `.env` file:

```bash
cp .env.example .env
```

After setting the `todo` policy instance in your Aserto account, update the the `.env` to contain the following values:

```
JWKS_URI=https://citadel.demo.aserto.com/dex/keys
ASERTO_AUTHORIZER_SERVICE_URL=localhost:8282
POLICY_ROOT="todoApp"
```

## Install dependencies

```bash
go get .
```

## Run the application

```bash
go run main.go
```
