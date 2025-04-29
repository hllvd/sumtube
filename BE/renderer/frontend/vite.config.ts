import { defineConfig } from "vite"
import react from "@vitejs/plugin-react"

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react()],
  build: {
    // Specify output directory
    outDir: "../static",
    // Clean the output directory before build
    emptyOutDir: true,
    rollupOptions: {
      output: {
        // Customize the output filenames
        entryFileNames: "app.js",
        chunkFileNames: "chunks/[name]-[hash].js",
        assetFileNames: "assets/[name]-[hash][extname]",
        // Bundle all code into a single file
        manualChunks: undefined,
      },
    },
    // Generate sourcemaps for debugging
    sourcemap: true,
    // Minify the output
    minify: "terser",
  },
})
