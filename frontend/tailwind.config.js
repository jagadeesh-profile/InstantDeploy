/** @type {import('tailwindcss').Config} */
export default {
  content: ["./index.html", "./src/**/*.{js,ts,jsx,tsx}"],
  theme: {
    extend: {
      colors: {
        primary: {
          50: "#eef7ff",
          100: "#d8ecff",
          200: "#b8dcff",
          300: "#86c4ff",
          400: "#4ca3ff",
          500: "#1f7bf0",
          600: "#0e61d2",
          700: "#114fab",
          800: "#15458d",
          900: "#173b75",
        },
        secondary: {
          50: "#fff7eb",
          100: "#ffedd1",
          200: "#ffd7a1",
          300: "#ffb861",
          400: "#ff9630",
          500: "#f97316",
          600: "#e25d0d",
          700: "#bb430f",
          800: "#953613",
          900: "#782f13",
        },
      },
      fontFamily: {
        display: ["Sora", "Space Grotesk", "Segoe UI", "sans-serif"],
        body: ["Space Grotesk", "Segoe UI", "sans-serif"],
        mono: ["IBM Plex Mono", "ui-monospace", "monospace"],
      },
      boxShadow: {
        glow: "0 16px 40px rgba(15, 56, 122, 0.2)",
      },
    },
  },
  plugins: [],
};
