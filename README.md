# Todo App with Aserto Authorization

This application demonstrates how to use Aserto's Go SDK to add an authorization layer to a simple todo app.

## Set up an `.env` file
Create the `.env` file:

```bash
cp .env.example .env
```

After setting the `todo` policy instance in your Aserto account / topaz, update the the `.env` to contain the following values:

### Env

```
JWKS_URI=https://citadel.demo.aserto.com/dex/keys
ISSUER=https://citadel.demo.aserto.com/dex
AUDIENCE=citadel-app

ASERTO_POLICY_ROOT="todoApp"

# Topaz
#
# This configuration targets a Topaz instance running locally.
# To target an Aserto hosted authorizer, comment out the lines below and uncomment the section
# at the bottom of this file.
ASERTO_AUTHORIZER_SERVICE_URL=localhost:8282
ASERTO_DIRECTORY_SERVICE_URL=localhost:9292
# On Windows, change these to '$HOMEPATH\AppData\Local\topaz\certs\grpc-ca.crt'
ASERTO_AUTHORIZER_CERT_PATH='${HOME}/.local/share/topaz/certs/grpc-ca.crt'
ASERTO_DIRECTORY_GRPC_CERT_PATH='$HOME/.local/share/topaz/certs/grpc-ca.crt'

# Aserto hosted authorizer
#
# To run the server using an Aserto hosted authorizer, the following variables are required:
# ASERTO_AUTHORIZER_SERVICE_URL=authorizer.prod.aserto.com:8443
# ASERTO_DIRECTORY_SERVICE_URL=directory.prod.aserto.com:8443
# ASERTO_TENANT_ID={Your Aserto Tenant ID UUID}
# ASERTO_AUTHORIZER_API_KEY={Your Authorizer API Key}
# ASERTO_DIRECTORY_API_KEY={Your Directory (read-only) API Key}
# ASERTO_POLICY_INSTANCE_NAME=todo
# ASERTO_POLICY_INSTANCE_LABEL=todo
```

## Install dependencies

```bash
go get .
```

## Run the application

```bash
go run main.go
```
