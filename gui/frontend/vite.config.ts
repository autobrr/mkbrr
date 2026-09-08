/*
 * Copyright (c) 2026, s0up4200 <s0up4200@pm.me> and the mkbrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import path from "path"
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
})
