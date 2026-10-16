import { useTranslation } from "react-i18next";

import { Button } from "@/shared/ui/button";
import {
  Card,
  CardContent,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/shared/ui/card";
import { WelcomeBrand } from "./welcome/welcome-brand";

export type StartupStage = "interface" | "connection" | "workspace";

export function StartupScreen({
  stage,
  error,
  onRetry,
  intro = true,
  onIntroComplete,
}: {
  stage: StartupStage;
  error?: string;
  onRetry?: () => void;
  intro?: boolean;
  onIntroComplete?: () => void;
}) {
  const { t } = useTranslation("common");
  return (
    <>
      <WelcomeBrand intro={intro} onIntroComplete={onIntroComplete}>
        {error ? (
          <Card>
            <CardHeader>
              <CardTitle>{t("startup.failed")}</CardTitle>
            </CardHeader>
            <CardContent>
              <p
                role="alert"
                className="text-destructive type-body break-words"
              >
                {error}
              </p>
            </CardContent>
            <CardFooter>
              <Button onClick={onRetry}>{t("startup.retry")}</Button>
            </CardFooter>
          </Card>
        ) : undefined}
      </WelcomeBrand>
      {!error && (
        <span className="sr-only" role="status" aria-live="polite">
          {t(`startup.status.${stage}`)}
        </span>
      )}
    </>
  );
}
