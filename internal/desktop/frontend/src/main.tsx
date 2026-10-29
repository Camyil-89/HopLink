import React from 'react'
import { createRoot } from 'react-dom/client'
import '@xyflow/react/dist/style.css'
import './assets/styles/app.css'
import './assets/styles/links.css'
import './assets/styles/layout.css'
import App from './app/App'

createRoot(document.getElementById('root')!).render(<React.StrictMode><App /></React.StrictMode>)
