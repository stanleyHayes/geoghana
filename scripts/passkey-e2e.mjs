#!/usr/bin/env node
/**
 * End-to-end WebAuthn ceremony against a running API (GEO-9.2).
 *
 * Uses Chrome's virtual authenticator over CDP, which is a real WebAuthn
 * implementation with no hardware — so this exercises the actual protocol
 * (attestation, assertion, signature verification) rather than a mock of it.
 * Nothing short of that proves a passkey flow works.
 *
 * The page is served from the API's own origin so the SameSite=Lax session
 * cookie is sent; a cross-origin page would silently drop it and the failure
 * would look like a bug in registration.
 *
 * Requires: an API on :8180 with passkeys enabled, and a verified account.
 *
 *   node scripts/passkey-e2e.mjs [email] [password]
 */
import { chromium } from "playwright";
const API = process.env.API_URL || "http://localhost:8180";
const EMAIL = process.argv[2] || "ama@example.com";
const PASSWORD = process.argv[3] || "kwabenya to osu every morning";
const b = await chromium.launch();
const ctx = await b.newContext();
const page = await ctx.newPage();
// Same origin as the API so the session cookie (SameSite=Lax) is sent.
await page.goto(`${API}/health`);

// Chrome's virtual authenticator: a real WebAuthn implementation with no hardware.
const cdp = await ctx.newCDPSession(page);
await cdp.send("WebAuthn.enable");
const { authenticatorId } = await cdp.send("WebAuthn.addVirtualAuthenticator", {
  options: { protocol: "ctap2", transport: "internal", hasResidentKey: true,
             hasUserVerification: true, isUserVerified: true, automaticPresenceSimulation: true },
});
console.log("  virtual authenticator:", authenticatorId.slice(0, 12) + "…");

const r = await page.evaluate(async ({ API, EMAIL, PASSWORD }) => {
  const j = (r) => r.json();
  const b64u = {
    dec: (s) => Uint8Array.from(atob(s.replace(/-/g,"+").replace(/_/g,"/")), c => c.charCodeAt(0)),
    enc: (b) => btoa(String.fromCharCode(...new Uint8Array(b))).replace(/\+/g,"-").replace(/\//g,"_").replace(/=+$/,""),
  };
  const out = {};

  // 1. Sign in with the password to get a session for enrolment.
  const login = await fetch(`${API}/v1/auth/login`, {
    method: "POST", credentials: "include", headers: {"Content-Type":"application/json"},
    body: JSON.stringify({ email: EMAIL, password: PASSWORD }),
  });
  out.login = login.status;

  // 2. Begin registration.
  const beginRes = await fetch(`${API}/v1/auth/passkeys/register/begin`, {
    method: "POST", credentials: "include", headers: {"Content-Type":"application/json"}, body: "{}",
  });
  out.registerBegin = beginRes.status;
  if (!beginRes.ok) { out.err = await beginRes.text(); return out; }
  const begin = await j(beginRes);

  const pk = begin.options.publicKey;
  pk.challenge = b64u.dec(pk.challenge);
  pk.user.id = b64u.dec(pk.user.id);
  if (pk.excludeCredentials) pk.excludeCredentials = pk.excludeCredentials.map(c => ({...c, id: b64u.dec(c.id)}));

  const cred = await navigator.credentials.create({ publicKey: pk });
  const attestation = {
    id: cred.id, rawId: b64u.enc(cred.rawId), type: cred.type,
    response: {
      clientDataJSON: b64u.enc(cred.response.clientDataJSON),
      attestationObject: b64u.enc(cred.response.attestationObject),
    },
    clientExtensionResults: cred.getClientExtensionResults(),
  };

  const finRes = await fetch(
    `${API}/v1/auth/passkeys/register/finish?challengeId=${encodeURIComponent(begin.challengeId)}&name=Test%20device`,
    { method: "POST", credentials: "include", headers: {"Content-Type":"application/json"},
      body: JSON.stringify(attestation) });
  out.registerFinish = finRes.status;
  out.registerBody = (await finRes.text()).slice(0, 160);

  // 3. Sign OUT, then sign in with the passkey alone.
  await fetch(`${API}/v1/auth/logout`, { method: "POST", credentials: "include" });

  const lbRes = await fetch(`${API}/v1/auth/passkeys/login/begin`, {
    method: "POST", credentials: "include", headers: {"Content-Type":"application/json"},
    body: JSON.stringify({ email: EMAIL }) });
  const lb = await j(lbRes);
  out.loginBegin = lbRes.status;
  out.allowCredentials = (lb.options.publicKey.allowCredentials || []).length;

  const apk = lb.options.publicKey;
  apk.challenge = b64u.dec(apk.challenge);
  if (apk.allowCredentials) apk.allowCredentials = apk.allowCredentials.map(c => ({...c, id: b64u.dec(c.id)}));

  const assertion = await navigator.credentials.get({ publicKey: apk });
  const body = {
    id: assertion.id, rawId: b64u.enc(assertion.rawId), type: assertion.type,
    response: {
      clientDataJSON: b64u.enc(assertion.response.clientDataJSON),
      authenticatorData: b64u.enc(assertion.response.authenticatorData),
      signature: b64u.enc(assertion.response.signature),
      userHandle: assertion.response.userHandle ? b64u.enc(assertion.response.userHandle) : null,
    },
    clientExtensionResults: assertion.getClientExtensionResults(),
  };
  const lfRes = await fetch(
    `${API}/v1/auth/passkeys/login/finish?challengeId=${encodeURIComponent(lb.challengeId)}`,
    { method: "POST", credentials: "include", headers: {"Content-Type":"application/json"},
      body: JSON.stringify(body) });
  out.loginFinish = lfRes.status;
  out.loginBody = (await lfRes.text()).slice(0, 120);

  // 4. The session from the passkey must work.
  const sess = await fetch(`${API}/v1/auth/session`, { credentials: "include" });
  out.session = sess.status;
  out.sessionBody = (await sess.text()).slice(0, 150);
  return out;
}, { API, EMAIL, PASSWORD });

for (const [k, v] of Object.entries(r)) console.log(`  ${k}: ${v}`);
await b.close();

// A non-zero exit so CI notices, rather than a green run with a 4xx inside it.
const failed = ["login","registerBegin","registerFinish","loginBegin","loginFinish","session"]
  .some((k) => typeof r[k] === "number" && r[k] >= 400);
if (failed || r.allowCredentials !== 1) {
  console.error("\n✗ passkey ceremony failed");
  process.exit(1);
}
console.log("\n✓ passkey registration and login verified with a real authenticator");
