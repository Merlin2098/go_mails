import { useState } from "react";
import "./App.css";
import Tabs, { type TabId } from "./components/Tabs";
import RenameTab from "./pages/Rename/RenameTab";
import SendTab from "./pages/Send/SendTab";
import SplashScreen from "./components/SplashScreen";

function App() {
  const [activeTab, setActiveTab] = useState<TabId>("rename");
  const [showSplash, setShowSplash] = useState(true);

  return (
    <div id="App">
      {showSplash && <SplashScreen onFinished={() => setShowSplash(false)} />}
      <header className="app-header">BoletaMailer</header>
      <div className="app-body">
        <Tabs active={activeTab} onChange={setActiveTab} />
        <main className="app-content">
          {activeTab === "rename" ? <RenameTab /> : <SendTab />}
        </main>
      </div>
    </div>
  );
}

export default App;
