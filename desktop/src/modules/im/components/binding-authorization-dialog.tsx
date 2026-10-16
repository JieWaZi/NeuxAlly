import { useTranslation } from "react-i18next";
import { Dialog, DialogContent } from "@/shared/ui/dialog";
import type { IMBinding } from "../types";
import { BindingForm } from "./binding-form";

export function BindingAuthorizationDialog({
  binding,
  onClose,
  onSaved,
}: {
  binding: IMBinding;
  onClose: () => void;
  onSaved: () => void;
}) {
  const { t } = useTranslation("im");
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <DialogContent title={t("authorize")}>
        <BindingForm
          purpose="authorization"
          binding={binding}
          onClose={onClose}
          onSaved={onSaved}
        />
      </DialogContent>
    </Dialog>
  );
}
