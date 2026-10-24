import { useState } from "react";
import { useTranslation } from "react-i18next";
import { QRCodeSVG } from "qrcode.react";
import { Button } from "@/shared/ui/button";
import { Dialog, DialogContent, DialogTrigger } from "@/shared/ui/dialog";

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
        <Button variant="secondary">{t("showQR")}</Button>
      </DialogTrigger>
      <DialogContent
        title={t("scanTitle")}
        description={code ? t("githubWaiting") : t("authWaiting")}
        size="sm"
      >
        <div className="space-y-4 overflow-auto px-6 pb-6">
          <div className="flex justify-center">
            <QRCodeSVG
              value={url}
              size={192}
              marginSize={4}
              level="M"
              title={t("scanTitle")}
            />
          </div>
          {code && <AuthorizationCode key={code} code={code} />}
        </div>
      </DialogContent>
    </Dialog>
  );
}
