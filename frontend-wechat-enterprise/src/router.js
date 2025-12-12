import { createRouter, createWebHashHistory } from 'vue-router';
import Login from './views/Login.js?v=5';
import Layout from './views/Layout.js?v=5';
import Chat from './views/Chat.js?v=5';
import Contacts from './views/Contacts.js?v=5';
import Profile from './views/Profile.js?v=5';

const routes = [
    { path: '/login', component: Login },
    { 
        path: '/', 
        component: Layout,
        children: [
            { path: '', redirect: '/chat' },
            { path: 'chat', component: Chat },
            { path: 'contacts', component: Contacts },
            { path: 'profile', component: Profile }
        ]
    }
];

export const router = createRouter({
    history: createWebHashHistory(),
    routes
});