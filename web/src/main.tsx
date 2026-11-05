import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { setToken, takeToken } from './api'
import { App, NoToken } from './App'
import { initTheme } from './theme'
import './styles.css'

initTheme()
const token = takeToken()
const root = createRoot(document.getElementById('root')!)

if (token) {
  setToken(token, () => root.render(<NoToken expired />))
  root.render(
    <StrictMode>
      <App />
    </StrictMode>,
  )
} else {
  root.render(<NoToken />)
}
