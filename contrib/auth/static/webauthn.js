// WebAuthn browser ceremony helpers for passkey registration and login.
"use strict";

// Get CSRF token from a hidden input on the page.
function getCsrfToken() {
  var el = document.getElementById("csrf-token");
  return el ? el.value : "";
}

// Base64url encoding/decoding helpers.
function base64urlEncode(buffer) {
  const bytes = new Uint8Array(buffer);
  let str = "";
  for (const b of bytes) str += String.fromCharCode(b);
  return btoa(str).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
}

function base64urlDecode(str) {
  str = str.replace(/-/g, "+").replace(/_/g, "/");
  while (str.length % 4) str += "=";
  const raw = atob(str);
  const bytes = new Uint8Array(raw.length);
  for (let i = 0; i < raw.length; i++) bytes[i] = raw.charCodeAt(i);
  return bytes.buffer;
}

// Convert server publicKey options for navigator.credentials.
function decodePublicKeyOptions(pk) {
  pk.challenge = base64urlDecode(pk.challenge);
  if (pk.user && pk.user.id) {
    pk.user.id = base64urlDecode(pk.user.id);
  }
  if (pk.excludeCredentials) {
    pk.excludeCredentials = pk.excludeCredentials.map(function (c) {
      c.id = base64urlDecode(c.id);
      return c;
    });
  }
  if (pk.allowCredentials) {
    pk.allowCredentials = pk.allowCredentials.map(function (c) {
      c.id = base64urlDecode(c.id);
      return c;
    });
  }
  return pk;
}

// Encode credential response for server.
function encodeAttestationResponse(cred) {
  return {
    id: cred.id,
    rawId: base64urlEncode(cred.rawId),
    type: cred.type,
    response: {
      attestationObject: base64urlEncode(cred.response.attestationObject),
      clientDataJSON: base64urlEncode(cred.response.clientDataJSON),
    },
  };
}

function encodeAssertionResponse(cred) {
  return {
    id: cred.id,
    rawId: base64urlEncode(cred.rawId),
    type: cred.type,
    response: {
      authenticatorData: base64urlEncode(cred.response.authenticatorData),
      clientDataJSON: base64urlEncode(cred.response.clientDataJSON),
      signature: base64urlEncode(cred.response.signature),
      userHandle: cred.response.userHandle
        ? base64urlEncode(cred.response.userHandle)
        : "",
    },
  };
}

// webauthnRegister performs the registration ceremony.
// beginURL: POST endpoint that returns {publicKey, user_id}
// finishURL: POST endpoint that receives the credential (user_id appended as query param)
// After success, shows recovery codes and optionally redirects.
async function webauthnRegister(beginURL, finishURL, formData) {
  var csrfToken = getCsrfToken();
  var beginResp = await fetch(beginURL, {
    method: "POST",
    headers: { "Content-Type": "application/json", "X-CSRF-Token": csrfToken },
    body: JSON.stringify(formData),
  });
  if (!beginResp.ok) {
    var err = await beginResp.json();
    throw new Error(err.error || "Registration failed");
  }
  var beginData = await beginResp.json();

  var publicKey = decodePublicKeyOptions(beginData.publicKey);
  var credential = await navigator.credentials.create({ publicKey: publicKey });

  var encoded = encodeAttestationResponse(credential);
  var finishResp = await fetch(
    finishURL + "?user_id=" + encodeURIComponent(beginData.user_id),
    {
      method: "POST",
      headers: { "Content-Type": "application/json", "X-CSRF-Token": csrfToken },
      body: JSON.stringify(encoded),
    },
  );
  if (!finishResp.ok) {
    var finishErr = await finishResp.json();
    throw new Error(finishErr.error || "Registration failed");
  }
  return await finishResp.json();
}

// webauthnLogin performs the discoverable login ceremony.
// beginURL: POST endpoint that returns {publicKey, session_id}
// finishURL: POST endpoint that receives the assertion (session_id appended as query param)
// Returns the server response (includes redirect).
async function webauthnLogin(beginURL, finishURL) {
  var csrfToken = getCsrfToken();
  var beginResp = await fetch(beginURL, {
    method: "POST",
    headers: { "X-CSRF-Token": csrfToken },
  });
  if (!beginResp.ok) {
    var err = await beginResp.json();
    throw new Error(err.error || "Login failed");
  }
  var beginData = await beginResp.json();

  var publicKey = decodePublicKeyOptions(beginData.publicKey);
  var credential = await navigator.credentials.get({ publicKey: publicKey });

  var encoded = encodeAssertionResponse(credential);
  var finishResp = await fetch(
    finishURL + "?session_id=" + encodeURIComponent(beginData.session_id),
    {
      method: "POST",
      headers: { "Content-Type": "application/json", "X-CSRF-Token": csrfToken },
      body: JSON.stringify(encoded),
    },
  );
  if (!finishResp.ok) {
    var finishErr = await finishResp.json();
    throw new Error(finishErr.error || "Login failed");
  }
  return await finishResp.json();
}

// webauthnAddCredential adds a new credential to an existing account.
// beginURL: POST endpoint that returns {publicKey}
// finishURL: POST endpoint that receives the credential
async function webauthnAddCredential(beginURL, finishURL) {
  var csrfToken = getCsrfToken();
  var beginResp = await fetch(beginURL, {
    method: "POST",
    headers: { "X-CSRF-Token": csrfToken },
  });
  if (!beginResp.ok) {
    var err = await beginResp.json();
    throw new Error(err.error || "Failed to start");
  }
  var beginData = await beginResp.json();

  var publicKey = decodePublicKeyOptions(beginData.publicKey);
  var credential = await navigator.credentials.create({ publicKey: publicKey });

  var encoded = encodeAttestationResponse(credential);
  var finishResp = await fetch(finishURL, {
    method: "POST",
    headers: { "Content-Type": "application/json", "X-CSRF-Token": csrfToken },
    body: JSON.stringify(encoded),
  });
  if (!finishResp.ok) {
    var finishErr = await finishResp.json();
    throw new Error(finishErr.error || "Failed to add credential");
  }
  return await finishResp.json();
}
