import type { Directive } from 'vue'

// Intersection Observer instance shared across all lazy-loaded images
let observer: IntersectionObserver | null = null

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
              img.src = src
              img.removeAttribute('data-src')
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
      // IntersectionObserver available - use lazy loading
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
