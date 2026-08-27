export type TabId = "rename" | "send";

interface TabsProps {
  active: TabId;
  onChange: (tab: TabId) => void;
}

const TAB_LABELS: Record<TabId, string> = {
  rename: "Renombrar",
  send: "Enviar",
};

export default function Tabs({ active, onChange }: TabsProps) {
  return (
    <nav className="tabs">
      {(Object.keys(TAB_LABELS) as TabId[]).map((id) => (
        <button
          key={id}
          className={`tab-button ${active === id ? "tab-button--active" : ""}`}
          onClick={() => onChange(id)}
        >
          {TAB_LABELS[id]}
        </button>
      ))}
    </nav>
  );
}
