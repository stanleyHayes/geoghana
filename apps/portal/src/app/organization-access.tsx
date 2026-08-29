"use client";

import {
  Check,
  Copy,
  Crown,
  MailPlus,
  RefreshCw,
  ShieldCheck,
  UserRoundCheck,
  X,
} from "lucide-react";
import { useEffect, useState } from "react";
import type { FormEvent } from "react";
import { Select } from "@ghanageo/ui";
import { portalRequest } from "./account-workspace";
import type { Organization } from "./account-workspace";

type Invitation = {
  id: string;
  email: string;
  role: "ADMIN" | "MEMBER" | "VIEWER";
  expiresAt: string;
};
export function OrganizationAccess({
  organization,
  currentAccountId,
  onChanged,
}: {
  organization: Organization;
  currentAccountId: string;
  onChanged: () => Promise<void>;
}) {
  const [invites, setInvites] = useState<Invitation[]>([]);
  const [token, setToken] = useState("");
  const [copied, setCopied] = useState(false);
  const [error, setError] = useState("");
  const currentMember = organization.members.find(
    (member) => member.accountId === currentAccountId,
  );
  const canInvite = ["OWNER", "ADMIN"].includes(currentMember?.role ?? "");
  const canTransfer = currentMember?.role === "OWNER";
  const load = async () => {
    try {
      const x = await portalRequest<{ data: Invitation[] }>(
        `/developer/organizations/${organization.id}/invitations`,
      );
      setInvites(x.data);
    } catch (e) {
      setError((e as Error).message);
    }
  };
  useEffect(() => {
    void load();
  }, [organization.id]);
  async function invite(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    const form = event.currentTarget;
    const data = new FormData(form);
    try {
      const x = await portalRequest<{ data: Invitation & { token: string } }>(
        `/developer/organizations/${organization.id}/invitations`,
        {
          method: "POST",
          body: JSON.stringify({
            email: data.get("email"),
            role: data.get("role"),
          }),
        },
      );
      setInvites((v) => [x.data, ...v]);
      setToken(x.data.token);
      setCopied(false);
      form.reset();
    } catch (e) {
      setError((e as Error).message);
    }
  }
  async function accept(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = event.currentTarget;
    const invitationToken = String(new FormData(form).get("token") || "");
    try {
      await portalRequest("/developer/invitations/accept", {
        method: "POST",
        body: JSON.stringify({ token: invitationToken }),
      });
      form.reset();
      await onChanged();
    } catch (e) {
      setError((e as Error).message);
    }
  }
  async function transfer(nextOwner: string) {
    if (
      !confirm(
        "Transfer ownership? You will become an administrator and only the new owner can transfer it again.",
      )
    )
      return;
    try {
      await portalRequest(
        `/developer/organizations/${organization.id}/transfer-ownership`,
        { method: "POST", body: JSON.stringify({ accountId: nextOwner }) },
      );
      await onChanged();
    } catch (e) {
      setError((e as Error).message);
    }
  }
  async function revoke(invitationId: string) {
    setError("");
    try {
      await portalRequest(
        `/developer/organizations/${organization.id}/invitations/${invitationId}/revoke`,
        { method: "POST" },
      );
      setInvites((current) =>
        current.filter((invitation) => invitation.id !== invitationId),
      );
    } catch (e) {
      setError((e as Error).message);
    }
  }
  return (
    <section className="organization-access">
      <div className="portal-section-head">
        <div>
          <p>Organization access</p>
          <h3>Members and invitations</h3>
        </div>
        <span>
          <ShieldCheck size={15} /> Role controlled
        </span>
      </div>
      {error ? <p className="account-message">{error}</p> : null}
      <div className="organization-access__grid">
        <div>
          <h4>Members</h4>
          {organization.members.map((member) => (
            <article key={member.accountId}>
              <span>
                <UserRoundCheck size={15} />
              </span>
              <div>
                <strong>{member.email || member.accountId}</strong>
                <small>{member.role}</small>
              </div>
              {organization.ownerId === member.accountId ? (
                <Crown size={15} />
              ) : canTransfer ? (
                <button
                  onClick={() => void transfer(member.accountId)}
                  title="Transfer ownership"
                >
                  <Crown size={14} />
                  <span>Make owner</span>
                </button>
              ) : null}
            </article>
          ))}
        </div>
        <div>
          <h4>Invite someone</h4>
          {canInvite ? (
            <form onSubmit={invite}>
              <input
                name="email"
                type="email"
                placeholder="person@example.com"
                aria-label="Invitation email"
                required
              />
              <Select
                name="role"
                ariaLabel="Invitation role"
                defaultValue="MEMBER"
                options={[
                  { value: "ADMIN", label: "Administrator", hint: "Manage members and applications" },
                  { value: "MEMBER", label: "Member", hint: "Work with organization resources" },
                  { value: "VIEWER", label: "Viewer", hint: "Read-only organization access" },
                ]}
              />
              <button className="portal-primary">
                <MailPlus size={15} /> Create invitation
              </button>
            </form>
          ) : (
            <p className="organization-access__hint">
              Owners and administrators manage invitations.
            </p>
          )}
          {token ? (
            <div className="invite-token">
              <code tabIndex={0}>{token}</code>
              <button
                onClick={async () => {
                  await navigator.clipboard.writeText(token);
                  setCopied(true);
                }}
              >
                {copied ? <Check size={14} /> : <Copy size={14} />}{" "}
                {copied ? "Copied" : "Copy token"}
              </button>
              <button
                aria-label="Dismiss invitation token"
                onClick={() => setToken("")}
              >
                <X size={14} />
              </button>
            </div>
          ) : null}
          <div className="invitation-list">
            {invites.map((v) => (
              <span key={v.id}>
                <span>
                  {v.email}
                  <small>
                    {v.role} · expires{" "}
                    {new Date(v.expiresAt).toLocaleDateString()}
                  </small>
                </span>
                {canInvite ? (
                  <button
                    aria-label={`Revoke invitation for ${v.email}`}
                    onClick={() => void revoke(v.id)}
                  >
                    <X size={13} />
                  </button>
                ) : null}
              </span>
            ))}
          </div>
        </div>
      </div>
      <form className="invite-accept" onSubmit={accept}>
        <div>
          <RefreshCw size={15} />
          <span>
            <strong>Have an invitation?</strong>
            <small>
              Paste the one-time token sent by the organization owner.
            </small>
          </span>
        </div>
        <input
          name="token"
          placeholder="Invitation token"
          aria-label="Invitation token"
          required
        />
        <button>Join organization</button>
      </form>
    </section>
  );
}
