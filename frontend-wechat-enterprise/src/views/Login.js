import { ref } from 'vue';
import { router } from '../router.js';
import { useMainStore } from '../store.js';
import { api } from '../api.js?v=5';

export default {
    setup() {
        
        const store = useMainStore();
        
        const isLogin = ref(true);
        const username = ref('');
        const password = ref('');
        const nickname = ref('');
        const errorMsg = ref('');
        const loading = ref(false);

        const toggleMode = () => {
            isLogin.value = !isLogin.value;
            errorMsg.value = '';
        };

        const handleSubmit = async () => {
            errorMsg.value = '';
            loading.value = true;
            try {
                if (isLogin.value) {
                    const res = await api.login(username.value, password.value);
                    // 登录接口返回 token，但不包含用户信息，需要单独获取
                    const token = res.token;
                    
                    const userData = await api.getCurrentUser(token);
                    
                    // 构造符合前端要求的用户对象
                    const user = {
                        id: userData.user_id,
                        username: userData.username,
                        nickname: userData.nickname,
                        avatar: `https://api.dicebear.com/7.x/avataaars/svg?seed=${userData.username}`
                    };

                    store.setUser(user, token);
                    router.push('/');
                } else {
                    await api.register(username.value, password.value, nickname.value);
                    alert('注册成功，请登录');
                    isLogin.value = true;
                }
            } catch (e) {
                errorMsg.value = e.message;
            } finally {
                loading.value = false;
            }
        };

        return {
            isLogin,
            username,
            password,
            nickname,
            errorMsg,
            loading,
            toggleMode,
            handleSubmit
        };
    },
    template: `
    <div class="min-h-screen flex items-center justify-center bg-gradient-to-br from-blue-500 to-indigo-600">
        <div class="bg-white p-8 rounded-2xl shadow-2xl w-96 transform transition-all hover:scale-105 duration-300">
            <div class="text-center mb-8">
                <div class="w-16 h-16 bg-blue-100 text-blue-600 rounded-full flex items-center justify-center mx-auto mb-4 text-2xl">
                    <i class="fas fa-comments"></i>
                </div>
                <h2 class="text-2xl font-bold text-gray-800">{{ isLogin ? '欢迎回来' : '创建账户' }}</h2>
                <p class="text-gray-500 text-sm mt-2">企业级即时通讯系统</p>
            </div>

            <form @submit.prevent="handleSubmit" class="space-y-4">
                <div>
                    <label class="block text-sm font-medium text-gray-700 mb-1">用户名</label>
                    <input v-model="username" type="text" required class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-transparent outline-none transition-all" placeholder="请输入用户名">
                </div>
                
                <div>
                    <label class="block text-sm font-medium text-gray-700 mb-1">密码</label>
                    <input v-model="password" type="password" required class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-transparent outline-none transition-all" placeholder="请输入密码">
                </div>

                <div v-if="!isLogin">
                    <label class="block text-sm font-medium text-gray-700 mb-1">昵称</label>
                    <input v-model="nickname" type="text" required class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-transparent outline-none transition-all" placeholder="请输入昵称">
                </div>

                <div v-if="errorMsg" class="text-red-500 text-sm text-center bg-red-50 p-2 rounded">
                    {{ errorMsg }}
                </div>

                <button type="submit" :disabled="loading" class="w-full bg-blue-600 hover:bg-blue-700 text-white font-semibold py-2 px-4 rounded-lg shadow transition-colors duration-200 flex justify-center items-center">
                    <i v-if="loading" class="fas fa-spinner fa-spin mr-2"></i>
                    {{ isLogin ? '登 录' : '注 册' }}
                </button>
            </form>

            <div class="mt-6 text-center text-sm text-gray-600">
                {{ isLogin ? '还没有账号？' : '已有账号？' }}
                <a href="#" @click.prevent="toggleMode" class="text-blue-600 hover:text-blue-800 font-medium">
                    {{ isLogin ? '立即注册' : '立即登录' }}
                </a>
            </div>
        </div>
    </div>
    `
};