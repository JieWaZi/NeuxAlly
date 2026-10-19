import { useState } from "react";
import { useTranslation } from "react-i18next";
import { QRCodeSVG } from "qrcode.react";
import { Button } from "@/shared/ui/button";
import { Dialog, DialogContent, DialogTrigger } from "@/shared/ui/dialog";
import { openExternalURL } from "@/shared/desktop";

/** 交互只依据桥接动作；普通浏览器链接不推导成可跨设备扫码的内容。 */
export function PluginAuthorizationActions({
  actions,
  onOpenError,
}: {
  actions: { type?: string; url?: string; value?: string }[];
  onOpenError: () => void;
}) {
  const { t } = useTranslation("plugin");
  return (
    <div className="space-y-3">
      {actions.map((action, index) => {
        const key = `${action.type}:${index}:${action.value ?? action.url ?? ""}`;
        if (action.type === "copy-code" && action.value)
          return <AuthorizationCode key={key} code={action.value} />;
        if (action.type === "show-qr" && action.value)
          return (
            <div key={key} className="space-y-3">
              <AuthorizationQRCode url={action.value} />
              <PluginAuthorizationDialog url={action.value} />
            </div>
          );
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

/** 关闭仅影响二维码展示，Core 授权等待继续；终态由父组件卸载本交互。 */
export function PluginAuthorizationDialog({
  url,
  code,
}: {
  url: string;
  code?: string;
}) {
  const { t } = useTranslation("plugin");
  return (
    <Dialog>
      <DialogTrigger asChild>
        <Button type="button" variant="secondary">
          {t("showQR")}
        </Button>
      </DialogTrigger>
      <DialogContent
        title={t("scanTitle")}
        description={t("authorizationWaiting")}
        size="sm"
      >
        <div className="space-y-4 overflow-auto px-6 pb-6">
          <AuthorizationQRCode url={url} />
          {code && <AuthorizationCode key={code} code={code} />}
        </div>
      </DialogContent>
    </Dialog>
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
