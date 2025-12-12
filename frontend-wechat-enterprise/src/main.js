import { createApp } from 'vue';
import { createPinia } from 'pinia';
import { router } from './router.js';
import { useMainStore } from './store.js';

const App = {
    template: `<router-view></router-view>`
};

const app = createApp(App);
const pinia = createPinia();

app.use(pinia);
app.use(router);

router.beforeEach((to, from, next) => {
    const store = useMainStore();
    if (to.path !== '/login' && !store.isLoggedIn) {
        next('/login');
    } else {
        next();
    }
});

app.mount('#app');