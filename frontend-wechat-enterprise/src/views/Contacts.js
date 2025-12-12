import { ref, onMounted, computed } from 'vue';
import { useMainStore } from '../store.js';
import { api } from '../api.js?v=5';
import { useRouter } from 'vue-router';

export default {
    setup() {
        const store = useMainStore();
        const router = useRouter();
        const activeTab = ref('friends'); // friends | groups
        const showAddModal = ref(false);
        const addType = ref('friend'); // friend | group
        const addInput = ref('');
        const addRemark = ref('');
        
        const friends = computed(() => store.friends);
        const groups = computed(() => store.groups);

        onMounted(() => {
            store.fetchContacts();
        });

        const openChat = (type, id, name) => {
            store.selectSession(type, id, name);
            router.push('/chat');
        };

        const handleAdd = async () => {
            try {
                if (addType.value === 'friend') {
                    await api.addFriend(addInput.value, addRemark.value, store.token);
                    alert('好友请求已发送');
                } else {
                    await api.createGroup(addInput.value, store.token);
                    alert('群组创建成功');
                }
                showAddModal.value = false;
                store.fetchContacts();
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
            openChat,
            handleAdd
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
            </div>
            <button @click="showAddModal = true" class="bg-green-500 hover:bg-green-600 text-white px-3 py-1 rounded text-sm">
                <i class="fas fa-plus mr-1"></i> 添加
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

            <div v-else class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                <div 
                    v-for="group in groups" 
                    :key="group.id"
                    class="flex items-center p-4 bg-white border border-gray-200 rounded-lg hover:shadow-md transition-shadow cursor-pointer"
                    @click="openChat('group', group.id, group.name)"
                >
                    <div class="w-12 h-12 rounded bg-purple-100 text-purple-600 flex items-center justify-center font-bold text-lg mr-4">
                        G
                    </div>
                    <div>
                        <h3 class="font-medium text-gray-900">{{ group.name }}</h3>
                        <p class="text-xs text-gray-500">ID: {{ group.id }}</p>
                    </div>
                </div>
            </div>
        </div>

        <!-- Modal -->
        <div v-if="showAddModal" class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
            <div class="bg-white rounded-lg p-6 w-96 shadow-xl">
                <h3 class="text-lg font-medium mb-4">添加联系人/群组</h3>
                
                <div class="flex space-x-4 mb-4">
                    <label class="flex items-center">
                        <input type="radio" v-model="addType" value="friend" class="mr-2"> 添加好友
                    </label>
                    <label class="flex items-center">
                        <input type="radio" v-model="addType" value="group" class="mr-2"> 创建群组
                    </label>
                </div>

                <div class="space-y-3">
                    <input 
                        v-model="addInput" 
                        type="text" 
                        class="w-full border border-gray-300 rounded px-3 py-2 text-sm"
                        :placeholder="addType === 'friend' ? '输入好友ID' : '输入群组名称'"
                    >
                    <input 
                        v-if="addType === 'friend'"
                        v-model="addRemark" 
                        type="text" 
                        class="w-full border border-gray-300 rounded px-3 py-2 text-sm"
                        placeholder="备注信息"
                    >
                </div>

                <div class="flex justify-end mt-6 space-x-3">
                    <button @click="showAddModal = false" class="px-4 py-2 text-gray-600 hover:bg-gray-100 rounded">取消</button>
                    <button @click="handleAdd" class="px-4 py-2 bg-green-500 text-white rounded hover:bg-green-600">确定</button>
                </div>
            </div>
        </div>
    </div>
    `
};