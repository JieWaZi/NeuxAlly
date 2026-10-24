import { useState } from "react";
import { Button } from "@/shared/ui/button";
import { logoutSession, type WebSession } from "../platform/session";
import { webText } from "../i18n";

export function SessionActions({ session }: { session: WebSession }) {
  const [pending, setPending] = useState(false);
  const [failed, setFailed] = useState(false);
  async function logout() {
    setPending(true);
    setFailed(false);
    try {
      await logoutSession(session);
    } catch {
      setPending(false);
      setFailed(true);
    }
  }
  return (
    <div className="flex items-center gap-2">
      {failed ? (
        <span className="type-caption text-destructive" role="alert">
          {webText("logoutFailure")}
        </span>
      ) : null}
      <Button
        size="sm"
        variant="ghost"
        disabled={pending}
        onClick={() => {
          void logout();
        }}
      >
        {webText("logout")}
      </Button>
    </div>
  );
}
