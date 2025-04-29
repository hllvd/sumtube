import { StrictMode } from "react"
import { createRoot } from "react-dom/client"
import App from "./App"
import "./index.css"

const el = document.getElementById("react-root")
if (el) {
  createRoot(el).render(
    <StrictMode>
      <App />
    </StrictMode>
  )
}
