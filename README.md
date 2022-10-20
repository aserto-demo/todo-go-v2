# Todo App with Aserto Authorization

This application demonstrates how to use Aserto's Go SDK to add an authorization layer to a simple todo app. For complete instructions on how to use the application, see this [tutorial](https://www.aserto.com/blog/adding-authorization-to-a-go-app-with-aserto).

## Set up an `.env` file
Create the `.env` file:

```bash
cp .env.example .env
```

After setting the `todo` policy instance in your Aserto account, retrieve the credentials from the policy settings tab:

```
AUTHORIZER_ADDRESS=authorizer.prod.aserto.com:8443
JWKS_URI=https://acmecorp.demo.aserto.com/dex/keys
POLICY_ROOT="todoApp"
AUTHORIZER_API_KEY=<Your Authorizer API Key>
POLICY_ID=<Your Policy ID>
TENANT_ID=<Your Tenant ID>
```

## Install dependencies

```bash
go get .
```

## Run the application

```bash
go run main.go
```