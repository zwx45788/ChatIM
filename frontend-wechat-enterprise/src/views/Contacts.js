import { ref, onMounted, computed } from 'vue';
import { useMainStore } from '../store.js';
import { api } from '../api.js?v=5';
import { router } from '../router.js';

export default {
    setup() {
        const store = useMainStore();
        
        const activeTab = ref('friends'); // friends | groups | search
        const showAddModal = ref(false);
        const addType = ref('friend'); // friend | group
        const addInput = ref('');
        const addRemark = ref('');
        const searchKeyword = ref('');
        const searchResults = ref([]);
        const searching = ref(false);
        
        const friends = computed(() => store.friends);
        const groups = computed(() => store.groups);

        onMounted(() => {
            store.fetchContacts();
        });

        const openChat = (type, id, name) => {
            store.selectSession(type, id, name);
            router.push('/chat');
        };
        
        const searchUsers = async () => {
            if (!searchKeyword.value.trim()) {
                searchResults.value = [];
                return;
            }
            searching.value = true;
            try {
                const res = await api.searchUsers(searchKeyword.value.trim(), 20, 0, store.token);
                searchResults.value = res.results || [];
            } catch (e) {
                alert('搜索失败: ' + e.message);
                searchResults.value = [];
            } finally {
                searching.value = false;
            }
        };

        const handleAdd = async () => {
            try {
                if (addType.value === 'friend') {
                    alert('好友功能暂未在后端实现，请使用搜索用户功能');
                    showAddModal.value = false;
                    return;
                } else {
                    if (!addInput.value.trim()) {
                        alert('请输入群组名称');
                        return;
                    }
                    await api.createGroup(addInput.value.trim(), '通过Web创建', '', store.token);
                    alert('群组创建成功');
                    addInput.value = '';
                }
                showAddModal.value = false;
                await store.fetchContacts();
            } catch (e) {
                alert('操作失败: ' + e.message);
            }
        };

        return {
            activeTab,
            friends,
            groups,
            showAddModal,
            addType,
            addInput,
            addRemark,
            searchKeyword,
            searchResults,
            searching,
            openChat,
            handleAdd,
            searchUsers
        };
    },
    template: `
    <div class="flex flex-col w-full h-full bg-white">
        <!-- Header -->
        <div class="h-16 border-b border-gray-200 flex items-center justify-between px-6 bg-gray-50">
            <div class="flex space-x-6">
                <button 
                    @click="activeTab = 'friends'"
                    class="text-sm font-medium pb-4 border-b-2 transition-colors"
                    :class="activeTab === 'friends' ? 'border-green-500 text-green-600' : 'border-transparent text-gray-500 hover:text-gray-700'"
                >
                    好友
                </button>
                <button 
                    @click="activeTab = 'groups'"
                    class="text-sm font-medium pb-4 border-b-2 transition-colors"
                    :class="activeTab === 'groups' ? 'border-green-500 text-green-600' : 'border-transparent text-gray-500 hover:text-gray-700'"
                >
                    群组
                </button>
                <button 
                    @click="activeTab = 'search'"
                    class="text-sm font-medium pb-4 border-b-2 transition-colors"
                    :class="activeTab === 'search' ? 'border-green-500 text-green-600' : 'border-transparent text-gray-500 hover:text-gray-700'"
                >
                    搜索用户
                </button>
            </div>
            <button @click="showAddModal = true" class="bg-green-500 hover:bg-green-600 text-white px-3 py-1 rounded text-sm">
                <i class="fas fa-plus mr-1"></i> 创建群组
            </button>
        </div>

        <!-- List -->
        <div class="flex-1 overflow-y-auto p-6">
            <div v-if="activeTab === 'friends'" class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                <div 
                    v-for="friend in friends" 
                    :key="friend.id"
                    class="flex items-center p-4 bg-white border border-gray-200 rounded-lg hover:shadow-md transition-shadow cursor-pointer"
                    @click="openChat('private', friend.id, friend.username)"
                >
                    <div class="w-12 h-12 rounded bg-blue-100 text-blue-600 flex items-center justify-center font-bold text-lg mr-4">
                        {{ friend.username.charAt(0).toUpperCase() }}
                    </div>
                    <div>
                        <h3 class="font-medium text-gray-900">{{ friend.username }}</h3>
                        <p class="text-xs text-gray-500">ID: {{ friend.id }}</p>
                    </div>
                </div>
            </div>

            <div v-else-if="activeTab === 'groups'" class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                <div 
                    v-for="group in groups" 
                    :key="group.group_id"
                    class="flex items-center p-4 bg-white border border-gray-200 rounded-lg hover:shadow-md transition-shadow cursor-pointer"
                    @click="openChat('group', group.group_id, group.name)"
                >
                    <div class="w-12 h-12 rounded bg-purple-100 text-purple-600 flex items-center justify-center font-bold text-lg mr-4">
                        G
                    </div>
                    <div>
                        <h3 class="font-medium text-gray-900">{{ group.name }}</h3>
                        <p class="text-xs text-gray-500">ID: {{ group.group_id }}</p>
                    </div>
                </div>
            </div>
            
            <div v-else class="space-y-4">
                <div class="flex gap-2">
                    <input 
                        v-model="searchKeyword" 
                        @keyup.enter="searchUsers"
                        type="text" 
                        placeholder="输入用户名搜索..."
                        class="flex-1 border border-gray-300 rounded-lg px-4 py-2"
                    >
                    <button 
                        @click="searchUsers" 
                        :disabled="searching"
                        class="bg-green-500 hover:bg-green-600 text-white px-6 py-2 rounded-lg disabled:opacity-50"
                    >
                        {{ searching ? '搜索中...' : '搜索' }}
                    </button>
                </div>
                
                <div v-if="searchResults.length === 0 && searchKeyword" class="text-center text-gray-500 py-8">
                    未找到用户
                </div>
                
                <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                    <div 
                        v-for="user in searchResults" 
                        :key="user.id"
                        class="flex items-center p-4 bg-white border border-gray-200 rounded-lg hover:shadow-md transition-shadow cursor-pointer"
                        @click="openChat('private', user.id, user.username)"
                    >
                        <div class="w-12 h-12 rounded bg-green-100 text-green-600 flex items-center justify-center font-bold text-lg mr-4">
                            {{ user.username.charAt(0).toUpperCase() }}
                        </div>
                        <div>
                            <h3 class="font-medium text-gray-900">{{ user.username }}</h3>
                            <p class="text-xs text-gray-500">{{ user.nickname || '无昵称' }}</p>
                            <p class="text-xs text-gray-400">ID: {{ user.id }}</p>
                        </div>
                    </div>
                </div>
            </div>
        </div>

        <!-- Modal -->
        <div v-if="showAddModal" class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
            <div class="bg-white rounded-lg p-6 w-96 shadow-xl">
                <h3 class="text-lg font-medium mb-4">创建群组</h3>

                <div class="space-y-3">
                    <input 
                        v-model="addInput" 
                        type="text" 
                        class="w-full border border-gray-300 rounded px-3 py-2 text-sm"
                        placeholder="输入群组名称"
                        @keyup.enter="handleAdd"
                    >
                </div>

                <div class="flex justify-end mt-6 space-x-3">
                    <button @click="showAddModal = false" class="px-4 py-2 text-gray-600 hover:bg-gray-100 rounded">取消</button>
                    <button @click="handleAdd" class="px-4 py-2 bg-green-500 text-white rounded hover:bg-green-600">创建</button>
                </div>
            </div>
        </div>
    </div>
    `
};