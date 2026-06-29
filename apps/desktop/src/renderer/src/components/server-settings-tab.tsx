import { useMemo, useState } from "react";
import { AlertCircle, Check, Loader2, RotateCcw } from "lucide-react";
import { toast } from "sonner";
import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "@multica/ui/components/ui/alert";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Label } from "@multica/ui/components/ui/label";
import { useT } from "@multica/views/i18n";
import {
  runtimeConfigFromInput,
  type RuntimeConfig,
} from "../../../shared/runtime-config";

type PreviewState =
  | { ok: true; config: RuntimeConfig }
  | { ok: false; error: string };

function errorMessage(err: unknown): string {
  return err instanceof Error ? err.message : String(err);
}

function previewConfig(apiUrl: string, appUrl: string): PreviewState {
  try {
    return {
      ok: true,
      config: runtimeConfigFromInput({
        apiUrl,
        appUrl: appUrl || undefined,
      }),
    };
  } catch (err) {
    return { ok: false, error: errorMessage(err) };
  }
}

function sameRuntimeConfig(a: RuntimeConfig | null, b: RuntimeConfig): boolean {
  return Boolean(
    a &&
      a.apiUrl === b.apiUrl &&
      a.wsUrl === b.wsUrl &&
      a.appUrl === b.appUrl,
  );
}

function EndpointRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="grid grid-cols-[120px_minmax(0,1fr)] items-baseline gap-3 py-1.5">
      <span className="text-xs text-muted-foreground">{label}</span>
      <span className="min-w-0 truncate font-mono text-xs" title={value}>
        {value}
      </span>
    </div>
  );
}

export function ServerSettingsTab() {
  const { t } = useT("settings");
  const initialConfig = window.desktopAPI.runtimeConfig.ok
    ? window.desktopAPI.runtimeConfig.config
    : null;
  const [savedConfig, setSavedConfig] = useState<RuntimeConfig | null>(null);
  const [savedPath, setSavedPath] = useState<string | null>(null);
  const [apiUrl, setApiUrl] = useState(initialConfig?.apiUrl ?? "");
  const [appUrl, setAppUrl] = useState(initialConfig?.appUrl ?? "");
  const [saving, setSaving] = useState(false);
  const baselineConfig = savedConfig ?? initialConfig;

  const preview = useMemo(
    () => previewConfig(apiUrl.trim(), appUrl.trim()),
    [apiUrl, appUrl],
  );
  const dirty = preview.ok && !sameRuntimeConfig(baselineConfig, preview.config);
  const canSave = preview.ok && dirty && !saving;

  async function handleSave() {
    if (!preview.ok || saving) return;
    setSaving(true);
    const result = await window.desktopAPI.saveRuntimeConfig({
      apiUrl: apiUrl.trim(),
      appUrl: appUrl.trim() || undefined,
    });
    setSaving(false);

    if (!result.ok) {
      toast.error(t(($) => $.desktop.server.save_failed_toast), {
        description: result.error,
      });
      return;
    }

    setSavedConfig(result.config);
    setSavedPath(result.path);
    setApiUrl(result.config.apiUrl);
    setAppUrl(result.config.appUrl);
    toast.success(t(($) => $.desktop.server.saved_toast));
  }

  function handleReset() {
    const next = baselineConfig ?? initialConfig;
    if (!next) return;
    setApiUrl(next.apiUrl);
    setAppUrl(next.appUrl);
  }

  return (
    <div>
      <h2 className="text-lg font-semibold">
        {t(($) => $.desktop.server.title)}
      </h2>
      <p className="mt-1 text-sm text-muted-foreground">
        {t(($) => $.desktop.server.description)}
      </p>

      <div className="mt-6 space-y-5">
        <div className="space-y-2">
          <Label htmlFor="desktop-api-url">
            {t(($) => $.desktop.server.api_url_label)}
          </Label>
          <Input
            id="desktop-api-url"
            value={apiUrl}
            onChange={(event) => setApiUrl(event.target.value)}
            placeholder="http://192.0.2.10:8080"
            aria-invalid={!preview.ok}
            spellCheck={false}
          />
          <p className="text-sm text-muted-foreground">
            {t(($) => $.desktop.server.api_url_description)}
          </p>
        </div>

        <div className="space-y-2">
          <Label htmlFor="desktop-app-url">
            {t(($) => $.desktop.server.app_url_label)}
          </Label>
          <Input
            id="desktop-app-url"
            value={appUrl}
            onChange={(event) => setAppUrl(event.target.value)}
            placeholder="http://192.0.2.10:3000"
            aria-invalid={!preview.ok}
            spellCheck={false}
          />
          <p className="text-sm text-muted-foreground">
            {t(($) => $.desktop.server.app_url_description)}
          </p>
          {!preview.ok && (apiUrl.trim().length > 0 || appUrl.trim().length > 0) && (
            <p className="inline-flex items-center gap-1.5 text-sm text-destructive">
              <AlertCircle className="size-3.5" />
              {preview.error}
            </p>
          )}
        </div>

        <div className="rounded-lg border bg-muted/20 px-4 py-3">
          <p className="text-sm font-medium">
            {t(($) => $.desktop.server.preview_title)}
          </p>
          <div className="mt-2">
            {preview.ok ? (
              <>
                <EndpointRow
                  label={t(($) => $.desktop.server.api_url)}
                  value={preview.config.apiUrl}
                />
                <EndpointRow
                  label={t(($) => $.desktop.server.app_url)}
                  value={preview.config.appUrl}
                />
                <EndpointRow
                  label={t(($) => $.desktop.server.websocket_url)}
                  value={preview.config.wsUrl}
                />
              </>
            ) : (
              <p className="text-sm text-muted-foreground">
                {t(($) => $.desktop.server.invalid_preview)}
              </p>
            )}
          </div>
        </div>

        {savedPath && (
          <Alert>
            <Check className="size-4" />
            <AlertTitle>
              {t(($) => $.desktop.server.restart_title)}
            </AlertTitle>
            <AlertDescription>
              {t(($) => $.desktop.server.restart_description, {
                path: savedPath,
              })}
            </AlertDescription>
          </Alert>
        )}

        <div className="flex justify-end gap-2">
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={handleReset}
            disabled={saving || !dirty}
          >
            <RotateCcw className="size-3.5" />
            {t(($) => $.desktop.server.reset)}
          </Button>
          <Button
            type="button"
            size="sm"
            onClick={handleSave}
            disabled={!canSave}
          >
            {saving ? (
              <Loader2 className="size-3.5 animate-spin" />
            ) : (
              <Check className="size-3.5" />
            )}
            {saving
              ? t(($) => $.desktop.server.saving)
              : t(($) => $.desktop.server.save)}
          </Button>
        </div>
      </div>
    </div>
  );
}
