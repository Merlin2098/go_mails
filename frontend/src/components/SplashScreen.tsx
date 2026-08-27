// Splash puramente visual/branding — no representa carga real (esta app no
// tiene lazy loading ni llamadas remotas al arrancar; todo el frontend está
// embebido estático en el binario). Duración fija, nunca atada a un estado
// de carga, para no fingir trabajo que no existe.
import { useEffect, useState } from "react";
import "./SplashScreen.css";

const SPLASH_DURATION_MS = 900;

function Gear({ className }: { className: string }) {
  return (
    <svg className={className} viewBox="0 0 100 100" xmlns="http://www.w3.org/2000/svg">
      <path
        fill="currentColor"
        d="M50 15a5 5 0 0 1 4.9 4l1.6 8.1a30 30 0 0 1 8.4 3.5l7-4.5a5 5 0 0 1 6.3.6l4 4a5 5 0 0 1 .6 6.3l-4.5 7a30 30 0 0 1 3.5 8.4l8.1 1.6a5 5 0 0 1 4 4.9v5.6a5 5 0 0 1-4 4.9l-8.1 1.6a30 30 0 0 1-3.5 8.4l4.5 7a5 5 0 0 1-.6 6.3l-4 4a5 5 0 0 1-6.3.6l-7-4.5a30 30 0 0 1-8.4 3.5l-1.6 8.1a5 5 0 0 1-4.9 4h-5.6a5 5 0 0 1-4.9-4l-1.6-8.1a30 30 0 0 1-8.4-3.5l-7 4.5a5 5 0 0 1-6.3-.6l-4-4a5 5 0 0 1-.6-6.3l4.5-7a30 30 0 0 1-3.5-8.4l-8.1-1.6a5 5 0 0 1-4-4.9v-5.6a5 5 0 0 1 4-4.9l8.1-1.6a30 30 0 0 1 3.5-8.4l-4.5-7a5 5 0 0 1 .6-6.3l4-4a5 5 0 0 1 6.3-.6l7 4.5a30 30 0 0 1 8.4-3.5l1.6-8.1a5 5 0 0 1 4.9-4z"
      />
      <circle cx="50" cy="50" r="18" fill="var(--ol-blue, #0078d4)" />
    </svg>
  );
}

export default function SplashScreen({ onFinished }: { onFinished: () => void }) {
  const [fadingOut, setFadingOut] = useState(false);

  useEffect(() => {
    const fadeTimer = setTimeout(() => setFadingOut(true), SPLASH_DURATION_MS - 200);
    const doneTimer = setTimeout(onFinished, SPLASH_DURATION_MS);
    return () => {
      clearTimeout(fadeTimer);
      clearTimeout(doneTimer);
    };
  }, [onFinished]);

  return (
    <div className={`splash ${fadingOut ? "splash--fading" : ""}`}>
      <div className="splash-gears">
        <Gear className="splash-gear splash-gear--large" />
        <Gear className="splash-gear splash-gear--small" />
      </div>
      <p className="splash-title">BoletaMailer</p>
    </div>
  );
}
