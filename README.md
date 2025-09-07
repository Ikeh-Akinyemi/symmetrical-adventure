# Gusto Webhook Handler

This project is a production-ready webhook handler for Gusto Embedded Payroll, written in Go. It serves as a reference implementation for building a resilient, secure, and fault-tolerant system that can handle the complexities of a real-world webhook integration.

The primary goal of this project is to demonstrate architectural best practices beyond a basic implementation.

-----

## Features

  * **Secure Signature Verification:** Verifies incoming webhooks using HMAC-SHA256 and a dynamic `verification_token` to prevent spoofing attacks.
  * **Asynchronous Processing:** Acknowledges webhook receipt immediately (`202 Accepted`) and processes events in the background using a worker pool to ensure high availability.
  * **Idempotency:** Prevents duplicate processing of retried events by tracking unique event UUIDs.
  * **Resilient Error Handling:** Intelligently classifies failures into transient vs. permanent and includes a **built-in retry mechanism** with backoff for transient processing errors.
  * **Integrated Setup:** Includes a local admin endpoint to orchestrate the multi-step webhook subscription and verification handshake with the Gusto API.

-----

## Project Structure

```plaintext
.
├── cmd/
│   └── server/
│       └── main.go
├── internal/
│   ├── contextkeys/
│   │   └── keys.go
│   ├── middleware/
│   │   └── security.go
│   ├── models/
│   │   └── types.go
│   ├── setup/
│   │   └── handler.go
│   ├── webhooks/
│   │   └── handler.go
│   └── worker/
│       ├── errors.go
│       ├── pool.go
│       └── store.go
├── .env
├── go.mod
└── Makefile
```

-----

## Prerequisites

Before you begin, ensure you have the following installed:

  * **Go** (version 1.21 or later)
  * **ngrok** (to expose your local server to the internet)
  * A **Gusto Developer Account** with an application created.

-----

## Setup and Configuration

**1. Clone the Repository**

```sh
git clone https://github.com/Ikeh-Akinyemi/symmetrical-adventure.git
cd symmetrical-adventure
```

**2. Create the Environment File**
Create a file named `.env` in the root of the project and populate it with the following variables:

```env
# The port for the HTTP server to listen on.
SERVER_PORT=8080

# Your Gusto API Token (get this from Step 3 of the Gusto Quickstart guide)
GUSTO_API_TOKEN="your_gusto_api_token_here"

# The verification token received from the API, used as the HMAC secret.
# This will be populated after running the /admin/setup-webhook endpoint.
GUSTO_VERIFICATION_TOKEN=""
```

**3. Get Your `GUSTO_API_TOKEN`**
This token is required to make administrative API calls to Gusto. Generate a `system_access_token` by following **Step 3** of the [Gusto Quickstart Guide](https://docs.gusto.com/embedded-payroll/docs/quickstart) and paste the `access_token` into your `.env` file. Note that this token expires after two hours.

-----

## Running the Application

The entire setup process requires three separate terminal windows.

**Terminal 1: Start the Server**

```sh
make run
```

The server will start and log a warning that the `GUSTO_VERIFICATION_TOKEN` is not yet set. This is expected.

**Terminal 2: Start ngrok**
Expose your local server to the internet.

```sh
ngrok http 8080
```

`ngrok` will provide a public HTTPS URL (e.g., `https://<random-string>.ngrok-free.app`). **Copy this URL.**

-----

## Webhook Subscription Setup

This is a one-time, two-part process to securely register and verify your webhook endpoint with Gusto.

**Step 1: Kick Off the Subscription**
In a third terminal, use `curl` to call the local `/admin/setup-webhook` endpoint. This tells your server to make an API call to Gusto to initiate the subscription. **Remember to replace `<YOUR_NGROK_URL>`** with the URL you copied from ngrok.

```sh
curl -X POST http://localhost:8080/admin/setup-webhook \
-H "Content-Type: application/json" \
-d '{"webhook_url": "https://<YOUR_NGROK_URL>/webhooks"}'
```

**Step 2: Get Verification Details from Logs**
After running the command, check the logs in **Terminal 1** (your running server). Gusto will have sent a verification payload to your endpoint, and your server will have logged the necessary details:

```json
{
  "level": "INFO",
  "msg": "✅ Received verification payload from Gusto. Use the token and UUID from the logs to complete verification.",
  "verification_token": "abc-123-token",
  "webhook_subscription_uuid": "xyz-456-uuid"
}
```

**Step 3: Complete the Verification**
Manually complete the handshake by using the `verification_token` and `webhook_subscription_uuid` from your logs in the following `curl` command. **Remember to replace the placeholders** with the values from your logs.

```sh
curl --request PUT \
  --url https://api.gusto-demo.com/v1/webhook_subscriptions/<PASTE_UUID_FROM_LOGS>/verify \
  --header 'Authorization: Bearer <YOUR_GUSTO_API_TOKEN>' \
  --header 'Content-Type: application/json' \
  --data '{
    "verification_token": "<PASTE_TOKEN_FROM_LOGS>"
  }'
```

**Step 4: Final Configuration**

1.  Copy the `verification_token` from your server logs and paste it into your `.env` file for the `GUSTO_VERIFICATION_TOKEN` variable.
2.  Restart your server (`Ctrl+C` and `make run`).

Your application is now fully configured and ready to receive webhooks securely.

-----

## Testing

To run the complete suite of unit tests, use the following command. The tests are run with the `-race` flag to detect potential concurrency issues.

```sh
make test
```

-----

## Makefile Commands

  * `make build`: Compiles the application binary.
  * `make run`: Runs the application locally.
  * `make test`: Runs all unit tests with the race detector.
  * `make lint`: Lints the codebase using `golangci-lint`.
  * `make clean`: Removes build artifacts.
  * `make help`: Displays a list of all available commands.