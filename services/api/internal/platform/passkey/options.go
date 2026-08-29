package passkey

import (
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

// credentialDescriptors lists the credentials already enrolled, so an
// authenticator refuses to register itself a second time.
func credentialDescriptors(u user) []protocol.CredentialDescriptor {
	creds := u.WebAuthnCredentials()
	out := make([]protocol.CredentialDescriptor, 0, len(creds))
	for _, c := range creds {
		out = append(out, c.Descriptor())
	}
	return out
}

// protocolResidentKeyPreferred asks for a discoverable credential without
// demanding one. Preferred rather than Required because a security key with
// no storage left would otherwise fail outright, and a non-discoverable
// passkey is still a strong second factor.
func protocolResidentKeyPreferred() protocol.ResidentKeyRequirement {
	return protocol.ResidentKeyRequirementPreferred
}

var _ = webauthn.Credential{}
