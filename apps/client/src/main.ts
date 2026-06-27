import { createApp } from "vue";

import "/@/assets/main.css";

import App from "./App.vue";
import { redirectOAuthCallback } from "./services/auth";

if (!redirectOAuthCallback()) {
    createApp(App).mount("#app");
}
