import { useState } from "react";
import { LoginPage } from "@multica/views/auth";
import { useT } from "@multica/views/i18n";
import { DragStrip } from "@multica/views/platform";
import { MulticaIcon } from "@multica/ui/components/common/multica-icon";
import { Button } from "@multica/ui/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogTitle,
} from "@multica/ui/components/ui/dialog";
import { Globe2 } from "lucide-react";
import { ServerSettingsTab } from "../components/server-settings-tab";

function requireRuntimeAppUrl(): string {
  const runtimeConfig = window.desktopAPI.runtimeConfig;
  if (!runtimeConfig.ok) {
    throw new Error(
      "Invariant violated: DesktopLoginPage rendered before App accepted runtime config",
    );
  }
  return runtimeConfig.config.appUrl;
}

function ServerSettingsLauncher() {
  const { t } = useT("settings");
  const [open, setOpen] = useState(false);
  const runtimeConfig = window.desktopAPI.runtimeConfig.ok
    ? window.desktopAPI.runtimeConfig.config
    : null;

  return (
    <>
      <Button
        type="button"
        variant="ghost"
        size="sm"
        className="w-full justify-center gap-2 text-muted-foreground"
        onClick={() => setOpen(true)}
      >
        <Globe2 className="size-3.5" />
        <span>{t(($) => $.desktop.tabs.server)}</span>
        {runtimeConfig && (
          <span
            className="min-w-0 truncate font-mono text-[11px]"
            title={runtimeConfig.apiUrl}
          >
            {runtimeConfig.apiUrl}
          </span>
        )}
      </Button>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className="sm:max-w-xl">
          <DialogTitle className="sr-only">
            {t(($) => $.desktop.server.title)}
          </DialogTitle>
          <ServerSettingsTab />
        </DialogContent>
      </Dialog>
    </>
  );
}

export function DesktopLoginPage() {
  const webUrl = requireRuntimeAppUrl();
  const handleGoogleLogin = () => {
    // Open web login page in the default browser with platform=desktop flag.
    // The web callback will redirect back via multica:// deep link with the token.
    window.desktopAPI.openExternal(
      `${webUrl}/login?platform=desktop`,
    );
  };

  return (
    <div className="flex h-screen flex-col">
      <DragStrip />
      <LoginPage
        logo={<MulticaIcon bordered size="lg" />}
        onSuccess={() => {
          // Auth store update triggers AppContent re-render → shows DesktopShell.
          // Initial workspace navigation happens in routes.tsx via IndexRedirect.
        }}
        onGoogleLogin={handleGoogleLogin}
        extra={<ServerSettingsLauncher />}
      />
    </div>
  );
}
