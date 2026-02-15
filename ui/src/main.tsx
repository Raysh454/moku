import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App'
import './styles.css'

type ErrorBoundaryState = {
  hasError: boolean
  message: string
}

class ErrorBoundary extends React.Component<React.PropsWithChildren, ErrorBoundaryState> {
  state: ErrorBoundaryState = {
    hasError: false,
    message: '',
  }

  static getDerivedStateFromError(error: unknown): ErrorBoundaryState {
    return {
      hasError: true,
      message: error instanceof Error ? error.message : String(error),
    }
  }

  componentDidCatch(error: unknown, info: React.ErrorInfo) {
    console.error('UI render crash:', error, info)
  }

  render() {
    if (this.state.hasError) {
      return (
        <div style={{ padding: 16, color: '#fecaca', background: '#7f1d1d', margin: 16, borderRadius: 8 }}>
          <h2 style={{ marginTop: 0 }}>UI crashed while rendering</h2>
          <p>{this.state.message || 'Unknown render error'}</p>
          <p>Open browser DevTools console for stack trace details.</p>
        </div>
      )
    }
    return this.props.children
  }
}

window.addEventListener('error', (event) => {
  console.error('Window error:', event.error || event.message)
})

window.addEventListener('unhandledrejection', (event) => {
  console.error('Unhandled promise rejection:', event.reason)
})

ReactDOM.createRoot(document.getElementById('root')!).render(
  <ErrorBoundary>
    <App />
  </ErrorBoundary>,
)
