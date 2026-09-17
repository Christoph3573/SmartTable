import { SettingsContent } from "../../components/settings/SettingsDialog";
import { PageHeader } from "../../components/ui/Page";

export function SettingsPage() {
  return (
    <div className="page">
      <PageHeader eyebrow="Anpassen" title="Einstellungen" />
      <section className="surface p-5">
        <SettingsContent />
      </section>
    </div>
  );
}
