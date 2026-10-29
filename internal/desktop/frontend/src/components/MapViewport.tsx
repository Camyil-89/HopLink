import { useEffect } from 'react'
import { useReactFlow, type Edge, type Node } from '@xyflow/react'

type Props = { nodes: Node[]; edges: Edge[]; revision: number }

export function MapViewport({ nodes, edges, revision }: Props) {
  const { fitView } = useReactFlow()
  const signature = `${nodes.map(node => node.id).join('|')}/${edges.map(edge => `${edge.source}>${edge.target}`).join('|')}/${revision}`

  useEffect(() => {
    if (!nodes.length) return

    let timer = 0
    const scheduleFit = () => {
      window.clearTimeout(timer)
      timer = window.setTimeout(() => {
        void fitView({ nodes, padding: 0.18, duration: 250, minZoom: 0.2, maxZoom: 1.4 })
      }, 80)
    }

    scheduleFit()
    const pane = document.querySelector('.flow-wrap')
    const observer = pane ? new ResizeObserver(scheduleFit) : null
    if (pane && observer) observer.observe(pane)
    return () => {
      window.clearTimeout(timer)
      observer?.disconnect()
    }
  }, [signature, fitView])

  return null
}
