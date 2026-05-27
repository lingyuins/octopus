import "./globals.css";
import { ThemeProvider } from "@/provider/theme";
import { Toaster } from "@/components/ui/sonner"
import { LocaleProvider } from "@/provider/locale";
import QueryProvider from "@/provider/query";
import { ServiceWorkerRegister } from "@/components/sw-register";
import { TooltipProvider } from "@/components/animate-ui/components/animate/tooltip";



export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html suppressHydrationWarning>
      <head>
        <meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover" />
        <meta name="theme-color" content="#faf8f5" />
        <meta name="application-name" content="Octopus" />
        <meta name="apple-mobile-web-app-capable" content="yes" />
        <meta name="apple-mobile-web-app-status-bar-style" content="black" />
        <meta name="apple-mobile-web-app-title" content="Octopus" />
        <meta name="mobile-web-app-capable" content="yes" />
        <meta name="mobile-web-app-status-bar-style" content="black" />
        <meta name="mobile-web-app-title" content="Octopus" />
        <link rel="manifest" href="./manifest.json" />
        <link rel="icon" href="./favicon.ico" sizes="any" />
        <link rel="apple-touch-icon" href="./apple-icon.png" />
        <title>Octopus</title>
        <style
          dangerouslySetInnerHTML={{
            __html: `
              #initial-loader {
                position: fixed;
                inset: 0;
                z-index: 9999;
                display: flex;
                align-items: center;
                justify-content: center;
                background: var(--background);
                color: var(--primary);
                transition: opacity 280ms ease;
              }
              #initial-loader.octo-hide {
                opacity: 0;
                pointer-events: none;
              }
              #initial-loader .octo-shell {
                display: grid;
                place-items: center;
                width: min(13rem, 40vw);
                aspect-ratio: 1;
                border-radius: 1rem;
                border: 1px solid var(--border);
                background: var(--card);
                box-shadow: var(--shadow-sm);
              }
              #initial-loader svg {
                position: relative;
                width: 7rem;
                height: 7rem;
              }
              #initial-loader .octo-group {
                animation: octoFade 2s ease-in-out infinite;
              }
              #initial-loader path {
                fill: none;
                stroke: currentColor;
                stroke-width: 6;
                stroke-linecap: round;
                stroke-dasharray: 1;
                stroke-dashoffset: 1;
                opacity: 0;
                animation: octoDraw 2s ease-in-out infinite both;
              }
              #initial-loader path:nth-child(1) { animation-delay: 0s; }
              #initial-loader path:nth-child(2) { animation-delay: 0.15s; }
              #initial-loader path:nth-child(3) { animation-delay: 0.30s; }
              #initial-loader path:nth-child(4) { animation-delay: 0.45s; }
              #initial-loader path:nth-child(5) { animation-delay: 0.60s; }

              @keyframes octoDraw {
                0%   { stroke-dashoffset: 1; opacity: 0; }
                5%   { opacity: 1; }
                40%  { stroke-dashoffset: 0; opacity: 1; }
                100% { stroke-dashoffset: 0; opacity: 1; }
              }
              @keyframes octoFade {
                0%   { opacity: 1; }
                70%  { opacity: 1; }
                100% { opacity: 0; }
              }

              @media (prefers-reduced-motion: reduce) {
                #initial-loader .octo-group,
                #initial-loader path {
                  animation: none !important;
                  opacity: 1 !important;
                  stroke-dashoffset: 0 !important;
                }
              }
            `,
          }}
        />
      </head>
      <body className="antialiased">
        <div id="initial-loader" role="status" aria-label="Loading application">
          <div className="octo-shell">
            <svg viewBox="0 0 100 100" xmlns="http://www.w3.org/2000/svg">
              <g className="octo-group">
                <path pathLength="1" d="M50 15 C70 15 85 30 85 50 C85 65 75 75 70 80 M50 15 C30 15 15 30 15 50 C15 65 25 75 30 80" />
                <path pathLength="1" d="M30 80 Q30 90 20 90" />
                <path pathLength="1" d="M43 77 Q43 90 38 90" />
                <path pathLength="1" d="M57 77 Q57 90 62 90" />
                <path pathLength="1" d="M70 80 Q70 90 80 90" />
              </g>
            </svg>
          </div>
        </div>
        <ServiceWorkerRegister />
        <ThemeProvider attribute="class" defaultTheme="system" enableSystem>
          <QueryProvider>
            <LocaleProvider>
              <TooltipProvider>
                {children}
                <Toaster />
              </TooltipProvider>
            </LocaleProvider>
          </QueryProvider>
        </ThemeProvider>
      </body>
    </html>
  );
}
