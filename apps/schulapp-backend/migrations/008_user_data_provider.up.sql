-- Daten-Provider pro Benutzer: legt fest, welche Schuldaten-Quelle der
-- Nutzer bevorzugt (Frontend-Umschalter "SmartTable/SchoolConnect"). Der
-- Wert wird serverseitig gespeichert und u. a. vom OpenCode-MCP respektiert.
ALTER TABLE users
  ADD COLUMN data_provider text NOT NULL DEFAULT 'smarttable'
  CHECK (data_provider IN ('smarttable', 'schoolconnect'));
