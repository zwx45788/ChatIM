import { useMainStore } from '../store.js';
import { useRouter } from 'vue-router';
import { computed } from 'vue';

export default {
    setup() {
        const store = useMainStore();
        const router = useRouter();
        const user = computed(() => store.user);

        const logout = () => {
            store.logout();
            router.push('/login');
        };

        return {
            user,
            logout
        };
    },
    template: `
    <div class="flex flex-col items-center justify-center w-full h-full bg-gray-50">
        <div class="bg-white p-8 rounded-xl shadow-lg w-96 text-center">
            <div class="w-24 h-24 bg-blue-500 rounded-full flex items-center justify-center text-white text-4xl font-bold mx-auto mb-6">
                {{ user?.username?.charAt(0).toUpperCase() }}
            </div>
            
            <h2 class="text-2xl font-bold text-gray-800 mb-2">{{ user?.username }}</h2>
            <p class="text-gray-500 mb-8">{{ user?.email || 'No email provided' }}</p>
            
            <div class="space-y-4">
                <div class="flex justify-between items-center p-3 bg-gray-50 rounded">
                    <span class="text-gray-600">用户 ID</span>
                    <span class="font-mono font-medium">{{ user?.id }}</span>
                </div>
                
                <button @click="logout" class="w-full bg-red-500 hover:bg-red-600 text-white font-medium py-2 px-4 rounded transition-colors">
                    退出登录
                </button>
            </div>
        </div>
    </div>
    `
};