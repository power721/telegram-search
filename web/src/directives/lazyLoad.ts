import type { Directive } from 'vue'

// Intersection Observer instance shared across all lazy-loaded images
let observer: IntersectionObserver | null = null

// Limit concurrent image loads to avoid browser connection limits
const MAX_CONCURRENT_LOADS = 6
let activeLoads = 0
const pendingLoads: Array<() => void> = []

function startLoad(img: HTMLImageElement, src: string) {
  if (activeLoads >= MAX_CONCURRENT_LOADS) {
    // Queue this load for later
    pendingLoads.push(() => startLoad(img, src))
    return
  }

  activeLoads++
  img.src = src
  img.removeAttribute('data-src')

  // When load completes (success or error), process next queued load
  const onComplete = () => {
    activeLoads--
    const next = pendingLoads.shift()
    if (next) next()
  }

  img.addEventListener('load', onComplete, { once: true })
  img.addEventListener('error', onComplete, { once: true })
}

function getObserver(): IntersectionObserver | null {
  // Check if IntersectionObserver is available (not available in jsdom/tests)
  if (typeof IntersectionObserver === 'undefined') {
    return null
  }

  if (!observer) {
    observer = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => {
          if (entry.isIntersecting) {
            const img = entry.target as HTMLImageElement
            const src = img.dataset.src
            if (src) {
              startLoad(img, src)
            }
            observer!.unobserve(img)
          }
        })
      },
      {
        rootMargin: '200px' // Start loading 200px before entering viewport
      }
    )
  }
  return observer
}

export const vLazyLoad: Directive<HTMLImageElement> = {
  mounted(el) {
    const src = el.dataset.src
    if (!src) return

    const obs = getObserver()
    if (obs) {
      // IntersectionObserver available - use lazy loading with concurrency control
      obs.observe(el)
    } else {
      // Fallback for environments without IntersectionObserver (tests, old browsers)
      el.src = src
      el.removeAttribute('data-src')
    }
  },
  unmounted(el) {
    if (observer) {
      observer.unobserve(el)
    }
  }
}
