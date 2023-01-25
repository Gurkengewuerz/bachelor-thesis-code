import './framework.scss'
import './style.scss'
import './clock.scss'
import * as bootstrap from 'bootstrap'

import App from './App.svelte'

const app = new App({
  target: document.getElementById('app')
})

export default app
