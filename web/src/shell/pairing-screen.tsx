import { useState, type FormEvent } from "react";
import { Button } from "@/shared/ui/button";
import { Input } from "@/shared/ui/input";
import { webText } from "../i18n";
import {
  requestSession,
  SessionError,
  type WebSession,
} from "../platform/session";

export function PairingScreen({
  onConnected,
  initialOffline = false,
}: {
  onConnected: (session: WebSession) => void;
  initialOffline?: boolean;
}) {
  const [code, setCode] = useState("");
  const [pending, setPending] = useState(false);
  const [error, setError] = useState(initialOffline ? webText("offline") : "");
  async function connect(event: FormEvent) {
    event.preventDefault();
    setPending(true);
    setError("");
    try {
      onConnected(await requestSession(code.trim()));
    } catch (reason) {
      setError(
        reason instanceof SessionError
          ? webText(
              reason.status === 429
                ? "limited"
                : reason.status === 401
                  ? "expired"
                  : reason.status === 403
                    ? "denied"
                    : reason.status === 409
                      ? "incompatible"
                      : "offline",
            )
          : webText("offline"),
      );
      setPending(false);
    }
  }
  return (
    <main className="bg-background text-foreground flex min-h-dvh items-center justify-center p-6">
      <form
        className="bg-surface-panel flex w-full max-w-md flex-col gap-4 rounded-xl p-6"
        onSubmit={(event) => {
          void connect(event);
        }}
      >
        <h1 className="type-page-title font-semibold">{webText("title")}</h1>
        <p className="type-body text-content-muted">{webText("description")}</p>
        <label
          className="type-label flex flex-col gap-2"
          htmlFor="pairing-code"
        >
          {webText("code")}
          <Input
            id="pairing-code"
            size="lg"
            autoFocus
            autoComplete="off"
            spellCheck={false}
            value={code}
            onChange={(event) => setCode(event.target.value)}
            placeholder={webText("placeholder")}
            maxLength={64}
            required
            disabled={pending}
            aria-invalid={Boolean(error)}
            aria-describedby={error ? "pairing-error" : undefined}
          />
        </label>
        {error ? (
          <p
            id="pairing-error"
            className="type-body text-destructive"
            role="alert"
          >
            {error}
          </p>
        ) : null}
        <Button type="submit" size="lg" disabled={pending || !code.trim()}>
          {webText(pending ? "connecting" : "pair")}
        </Button>
      </form>
    </main>
  );
}
