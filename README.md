# API

## Auth

Auth is done via SSH-Key signing, supported SSH-Keys are: eddsa25519
Headers for authentification:
- `X-Signature`: base 64 encoded signiture of "method|path|timestamp" e.g: "GET|/hello|1111111111"
- `X-Timestamp`: timestamp of the request
- `X-User-ID`: id of ther you want to authentifikate with
