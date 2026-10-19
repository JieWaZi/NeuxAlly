import { useState } from "react";
import { useTranslation } from "react-i18next";
import { QRCodeSVG } from "qrcode.react";
import { Button } from "@/shared/ui/button";
import { RefreshCw } from "lucide-react";
import { Spinner } from "@/shared/ui/spinner";
import { openExternalURL } from "@/shared/desktop";

/** 交互只依据桥接动作；普通浏览器链接不推导成可跨设备扫码的内容。 */
export function PluginAuthorizationActions({
  actions,
  onOpenError,
  onRefresh,
  disabled = false,
  qrState,
}: {
  actions: { type?: string; url?: string; value?: string }[];
  onOpenError: () => void;
  onRefresh: () => void;
  disabled?: boolean;
  qrState?: "loading" | "expired";
}) {
  const { t } = useTranslation("plugin");
  const qr = actions.find(
    (action) => action.type === "show-qr" && action.value,
  );
  return (
    <div className="space-y-3">
      {(qr || qrState) && (
        <div className="flex items-start justify-center gap-2">
          <div className="bg-surface-subtle grid size-48 place-items-center rounded-md">
            {qrState ? (
              <div className="flex flex-col items-center gap-3">
                {qrState === "loading" && (
                  <Spinner aria-label={t("qrLoading")} />
                )}
                <span className="type-caption text-content-secondary">
                  {t(qrState === "loading" ? "qrLoading" : "qrExpired")}
                </span>
              </div>
            ) : (
              <AuthorizationQRCode url={qr?.value ?? ""} />
            )}
          </div>
          <Button
            type="button"
            variant="ghost"
            size="icon"
            aria-label={t("refreshQR")}
            title={t("refreshQR")}
            disabled={disabled || qrState === "loading"}
            onClick={onRefresh}
          >
            <RefreshCw aria-hidden="true" />
          </Button>
        </div>
      )}
      {actions.map((action, index) => {
        const key = `${action.type}:${index}:${action.value ?? action.url ?? ""}`;
        if (action.type === "copy-code" && action.value)
          return <AuthorizationCode key={key} code={action.value} />;
        if (
          action.type === "open-url" &&
          action.url &&
          /^https?:\/\//.test(action.url)
        )
          return (
            <Button
              key={key}
              type="button"
              variant="secondary"
              disabled={disabled}
              onClick={() =>
                void openExternalURL(action.url ?? "").catch(onOpenError)
              }
            >
              {t("openAuth")}
            </Button>
          );
        return null;
      })}
    </div>
  );
}

/** 验证码是官方要求用户输入的短时信息，不能拼进未经官方提供的 URL。 */
export function AuthorizationCode({ code }: { code: string }) {
  const { t } = useTranslation("plugin");
  const [copied, setCopied] = useState(false);
  const [failed, setFailed] = useState(false);
  return (
    <div className="space-y-2">
      <p className="type-body text-muted-foreground">{t("deviceCodeHelp")}</p>
      <div className="flex flex-wrap items-center gap-3">
        <code className="type-title font-semibold">{code}</code>
        <Button
          type="button"
          variant="secondary"
          onClick={() => {
            setFailed(false);
            void Promise.resolve()
              .then(() => navigator.clipboard.writeText(code))
              .then(
                () => setCopied(true),
                () => setFailed(true),
              );
          }}
        >
          {copied ? t("codeCopied") : t("copyCode")}
        </Button>
      </div>
      {failed && (
        <p role="alert" className="type-body text-destructive">
          {t("copyCodeFailed")}
        </p>
      )}
    </div>
  );
}

/** 二维码保持原始官方 URL；浅色底保证暗色主题下仍可识别。 */
export function AuthorizationQRCode({ url }: { url: string }) {
  const { t } = useTranslation("plugin");
  return (
    <div className="flex justify-center">
      <QRCodeSVG
        value={url}
        size={192}
        marginSize={4}
        level="M"
        title={t("scanTitle")}
      />
    </div>
  );
}
