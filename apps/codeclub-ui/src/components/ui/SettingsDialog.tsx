import { useEffect, useState } from "react";
import { Button } from "./Button";
import { useSettingsStore, type DataSource } from "../../store/settingsStore";

const options: { value: DataSource; label: string; description: string }[] = [
  {
    value: "proprietary",
    label: "Proprietär",
    description: "Eigene SmartTable-Datenhaltung (Standard).",
  },
  {
    value: "schoolconnect",
    label: "SchoolConnect",
    description: "Externe SchoolConnect-Anbindung.",
  },
];

export function SettingsDialog({ onClose }: { onClose: () => void }) {
  const dataSource = useSettingsStore((state) => state.dataSource);
  const setDataSource = useSettingsStore((state) => state.setDataSource);
  const [selection, setSelection] = useState<DataSource>(dataSource);

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [onClose]);

  const handleSave = () => {
    setDataSource(selection);
    onClose();
  };

  return (
    <div
      className="modal-backdrop"
      role="dialog"
      aria-modal="true"
      aria-label="Einstellungen"
      onClick={onClose}
    >
      <div className="modal" onClick={(event) => event.stopPropagation()}>
        <h2>Einstellungen</h2>
        <p className="mb-4 text-sm text-gray-500 dark:text-gray-400">
          Wähle die Datenquelle für SmartTable.
        </p>
        <fieldset>
          <legend className="mb-2 text-sm font-medium text-gray-700 dark:text-gray-200">
            Datenquelle
          </legend>
          <div className="flex flex-col gap-2">
            {options.map((option) => (
              <label
                key={option.value}
                className={`flex cursor-pointer items-start gap-3 rounded-lg border px-3 py-2.5 text-sm transition-colors ${
                  selection === option.value
                    ? "border-indigo-600 bg-indigo-50 dark:border-indigo-400 dark:bg-indigo-900/30"
                    : "border-gray-300 hover:border-gray-400 dark:border-gray-600"
                }`}
              >
                <input
                  type="radio"
                  name="data-source"
                  className="mt-1 accent-indigo-600"
                  checked={selection === option.value}
                  onChange={() => setSelection(option.value)}
                />
                <span>
                  <span className="block font-medium text-gray-900 dark:text-white">
                    {option.label}
                  </span>
                  <span className="mt-0.5 block text-xs text-gray-500 dark:text-gray-400">
                    {option.description}
                  </span>
                </span>
              </label>
            ))}
          </div>
        </fieldset>
        <div className="modal-actions">
          <Button type="button" variant="ghost" onClick={onClose}>
            Abbrechen
          </Button>
          <Button type="button" onClick={handleSave}>
            Speichern
          </Button>
        </div>
      </div>
    </div>
  );
}
