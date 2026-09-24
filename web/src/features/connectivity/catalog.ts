import raw from '../../../public/connectivity-targets.json'
import type { Target } from './engine'

// The checked-in catalog is validated by catalog.test.mjs and the Go CSP tests.
export const targets = raw as Target[]
