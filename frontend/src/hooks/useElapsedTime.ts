import { useEffect, useRef, useState } from "react";

// Cuenta el tiempo transcurrido en segundos mientras running es true.
// Se congela en el último valor al pasar a false — no sigue avanzando
// después de que un proceso termina — y vuelve a 0 la siguiente vez que
// running pasa de false a true (un nuevo lote empieza desde cero).
export function useElapsedTime(running: boolean): number {
  const [elapsedMs, setElapsedMs] = useState(0);
  const startedAtRef = useRef<number | null>(null);

  useEffect(() => {
    if (!running) {
      startedAtRef.current = null;
      return;
    }

    startedAtRef.current = performance.now();
    setElapsedMs(0);

    const intervalId = setInterval(() => {
      if (startedAtRef.current !== null) {
        setElapsedMs(performance.now() - startedAtRef.current);
      }
    }, 250);

    return () => clearInterval(intervalId);
  }, [running]);

  return Math.floor(elapsedMs / 1000);
}

export function formatElapsed(totalSeconds: number): string {
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  return `${String(minutes).padStart(2, "0")}:${String(seconds).padStart(2, "0")}`;
}
