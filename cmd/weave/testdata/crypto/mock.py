#!/usr/bin/env python3
import base64
import hashlib
import hmac
import json
import os
import sys

PROVIDER = "mock"

if len(sys.argv) > 1 and sys.argv[1] == "--capabilities":
    key_ids = os.environ.get("MOCK_KEY_IDS", "ops-prod-key,tenant-salt")
    print(json.dumps({
        "v": "weave.cli/v1",
        "type": "crypto_capabilities",
        "provider": PROVIDER,
        "operations": ["encrypt", "decrypt", "hash", "tokenize"],
        "algorithms": ["aes-256-gcm", "hmac-sha256"],
        "key_ids": [key for key in key_ids.split(",") if key],
    }))
    sys.exit(0)

if os.environ.get("MOCK_BROKEN") == "1":
    print("not json")
    sys.exit(0)

request = json.load(sys.stdin)
operation = request["operation"]
response = {
    "v": "weave.cli/v1",
    "type": "crypto_response",
    "operation": operation,
    "provider": PROVIDER,
    "key_id": request["key_id"],
    "key_version": 3,
}

if operation == "encrypt":
    payload = base64.b64encode(request["plaintext"].encode()).decode()
    response["ciphertext"] = "mock:v1:" + payload
elif operation == "decrypt":
    ciphertext = request["ciphertext"]
    if not ciphertext.startswith("mock:v1:"):
        sys.exit(1)
    response["plaintext"] = base64.b64decode(ciphertext[len("mock:v1:"):]).decode()
elif operation == "tokenize":
    digest = hmac.new(b"mock", request["value"].encode(), hashlib.sha256).hexdigest()
    response["token"] = "tok" + digest[:16]
elif operation == "hash":
    response["digest"] = "hash" + hashlib.sha256(request["value"].encode()).hexdigest()[:16]
else:
    sys.exit(1)

print(json.dumps(response))
