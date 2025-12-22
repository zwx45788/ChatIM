import { useMainStore } from '../store.js';
import { router } from '../router.js';
import { computed } from 'vue';

export default {
    setup() {
        const store = useMainStore();
        
        const route = router.currentRoute;
        
        const user = computed(() => store.user);
        const currentPath = computed(() => route.value?.path || '/');

        const navItems = [
            { path: '/chat', icon: 'fas fa-comment', label: '聊天' },
            { path: '/contacts', icon: 'fas fa-address-book', label: '通讯录' },
            { path: '/profile', icon: 'fas fa-user', label: '我' }
        ];

        const navigate = (path) => {
            router.push(path);
        };

        return {
            user,
            navItems,
            currentPath,
            navigate
        };
    },
    template: `
    <div class="flex h-screen bg-gray-100">
        <!-- Sidebar -->
        <div class="w-16 bg-slate-900 flex flex-col items-center py-6 space-y-8 text-gray-400">
            <!-- Avatar -->
            <div class="w-10 h-10 rounded bg-blue-500 flex items-center justify-center text-white font-bold text-lg mb-4 cursor-pointer" @click="navigate('/profile')">
                {{ user?.username?.charAt(0).toUpperCase() }}
            </div>

            <!-- Nav Items -->
            <div 
                v-for="item in navItems" 
                :key="item.path"
                @click="navigate(item.path)"
                class="w-10 h-10 flex items-center justify-center rounded-lg cursor-pointer transition-all duration-200 hover:text-white"
                :class="currentPath.includes(item.path) ? 'text-green-400' : ''"
                :title="item.label"
            >
                <i :class="item.icon + ' text-xl'"></i>
            </div>

            <div class="flex-grow"></div>

            <!-- Settings/Logout -->
            <div class="w-10 h-10 flex items-center justify-center rounded-lg cursor-pointer hover:text-white transition-all duration-200">
                <i class="fas fa-cog text-xl"></i>
            </div>
        </div>

        <!-- Main Content -->
        <div class="flex-1 flex overflow-hidden">
            <router-view></router-view>
        </div>
    </div>
    `
};