import { ref, computed, onMounted, onUnmounted, nextTick, watch } from 'vue';
import { useMainStore } from '../store.js';
import { api } from '../api.js?v=5';

export default {
    setup() {
        const store = useMainStore();
        const messageInput = ref('');
        const messagesContainer = ref(null);
        const searchKeyword = ref('');
        
        const currentSession = computed(() => store.currentSession);
        const messages = computed(() => store.currentMessages);
        const friends = computed(() => store.friends);
        const groups = computed(() => store.groups);
        
        const sessionList = computed(() => {
            const list = [];
            friends.value.forEach(f => {
                if (f.username.includes(searchKeyword.value)) {
                    list.push({
                        type: 'private',
                        id: f.id,
                        name: f.username,
                        avatar: f.username.charAt(0).toUpperCase(),
                        unread: store.unreadCounts[`private_${f.id}`] || 0
                    });
                }
            });
            groups.value.forEach(g => {
                if (g.name.includes(searchKeyword.value)) {
                    list.push({
                        type: 'group',
                        id: g.id,
                        name: g.name,
                        avatar: 'G',
                        unread: store.unreadCounts[`group_${g.id}`] || 0
                    });
                }
            });
            return list;
        });

        const scrollToBottom = () => {
            nextTick(() => {
                if (messagesContainer.value) {
                    messagesContainer.value.scrollTop = messagesContainer.value.scrollHeight;
                }
            });
        };

        watch(messages, () => {
            scrollToBottom();
        }, { deep: true });
        
        watch(currentSession, () => {
            scrollToBottom();
        });

        const selectSession = (item) => {
            store.selectSession(item.type, item.id, item.name);
        };

        const sendMessage = async () => {
            if (!messageInput.value.trim()) return;
            try {
                await store.sendMessage(messageInput.value);
                messageInput.value = '';
            } catch (e) {
                alert('发送失败: ' + e.message);
            }
        };

        let pollTimer = null;

        onMounted(async () => {
            await store.fetchContacts();
            await store.pullMessages();
            
            pollTimer = setInterval(() => {
                store.pullMessages();
            }, 3000);
        });

        onUnmounted(() => {
            if (pollTimer) clearInterval(pollTimer);
        });

        const formatTime = (timeStr) => {
            if (!timeStr) return '';
            const date = new Date(timeStr);
            return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
        };

        return {
            store,
            messageInput,
            messagesContainer,
            searchKeyword,
            currentSession,
            messages,
            sessionList,
            selectSession,
            sendMessage,
            formatTime
        };
    },
    template: `
    <div class="flex w-full h-full bg-white">
        <!-- Session List (Left) -->
        <div class="w-80 border-r border-gray-200 flex flex-col bg-gray-50">
            <!-- Search -->
            <div class="p-4 bg-gray-100 border-b border-gray-200">
                <div class="relative">
                    <i class="fas fa-search absolute left-3 top-3 text-gray-400"></i>
                    <input v-model="searchKeyword" type="text" class="w-full pl-10 pr-4 py-2 bg-white border border-gray-300 rounded text-sm focus:outline-none focus:border-green-500" placeholder="搜索">
                    <button class="absolute right-2 top-2 bg-gray-200 p-1 rounded text-gray-600 hover:bg-gray-300 text-xs">
                        <i class="fas fa-plus"></i>
                    </button>
                </div>
            </div>

            <!-- List -->
            <div class="flex-1 overflow-y-auto">
                <div 
                    v-for="item in sessionList" 
                    :key="item.type + '_' + item.id"
                    @click="selectSession(item)"
                    class="flex items-center p-3 cursor-pointer hover:bg-gray-200 transition-colors"
                    :class="currentSession && currentSession.type === item.type && currentSession.id === item.id ? 'bg-gray-200' : ''"
                >
                    <div class="relative">
                        <div class="w-10 h-10 rounded bg-blue-500 flex items-center justify-center text-white font-bold mr-3">
                            {{ item.avatar }}
                        </div>
                        <div v-if="item.unread > 0" class="absolute -top-1 -right-1 bg-red-500 text-white text-xs rounded-full w-4 h-4 flex items-center justify-center">
                            {{ item.unread }}
                        </div>
                    </div>
                    <div class="flex-1 min-w-0">
                        <div class="flex justify-between items-baseline">
                            <h3 class="text-sm font-medium text-gray-900 truncate">{{ item.name }}</h3>
                            <span class="text-xs text-gray-400">12:30</span>
                        </div>
                        <p class="text-xs text-gray-500 truncate">点击开始聊天</p>
                    </div>
                </div>
            </div>
        </div>

        <!-- Chat Area (Right) -->
        <div class="flex-1 flex flex-col bg-gray-50" v-if="currentSession">
            <!-- Header -->
            <div class="h-16 border-b border-gray-200 flex items-center justify-between px-6 bg-white">
                <h2 class="text-lg font-medium text-gray-800">{{ currentSession.name }}</h2>
                <i class="fas fa-ellipsis-h text-gray-500 cursor-pointer hover:text-gray-700"></i>
            </div>

            <!-- Messages -->
            <div class="flex-1 overflow-y-auto p-6 space-y-4" ref="messagesContainer">
                <div 
                    v-for="msg in messages" 
                    :key="msg.id" 
                    class="flex"
                    :class="msg.is_self ? 'justify-end' : 'justify-start'"
                >
                    <div class="flex max-w-[70%] items-end" :class="msg.is_self ? 'flex-row-reverse' : 'flex-row'">
                        <!-- Avatar -->
                        <div class="w-8 h-8 rounded bg-gray-300 flex-shrink-0 flex items-center justify-center text-xs text-gray-600 font-bold mx-2">
                            {{ msg.is_self ? store.user.username.charAt(0).toUpperCase() : (currentSession.type === 'group' ? '?' : currentSession.name.charAt(0).toUpperCase()) }}
                        </div>
                        
                        <!-- Bubble -->
                        <div 
                            class="px-4 py-2 rounded-lg shadow-sm text-sm break-words relative group"
                            :class="msg.is_self ? 'bg-green-500 text-white rounded-br-none' : 'bg-white text-gray-800 rounded-bl-none'"
                        >
                            {{ msg.content }}
                            <div class="text-[10px] opacity-70 mt-1 text-right" :class="msg.is_self ? 'text-green-100' : 'text-gray-400'">
                                {{ formatTime(msg.created_at) }}
                            </div>
                        </div>
                    </div>
                </div>
            </div>

            <!-- Input -->
            <div class="h-40 border-t border-gray-200 bg-white p-4 flex flex-col">
                <div class="flex space-x-4 text-gray-500 mb-2 px-2">
                    <i class="far fa-smile cursor-pointer hover:text-gray-700"></i>
                    <i class="far fa-folder cursor-pointer hover:text-gray-700"></i>
                    <i class="fas fa-cut cursor-pointer hover:text-gray-700"></i>
                </div>
                <textarea 
                    v-model="messageInput"
                    @keydown.enter.prevent="sendMessage"
                    class="flex-1 resize-none outline-none text-sm text-gray-700 bg-transparent"
                    placeholder="输入消息..."
                ></textarea>
                <div class="flex justify-end mt-2">
                    <button @click="sendMessage" class="bg-gray-100 hover:bg-green-500 hover:text-white text-green-600 px-6 py-1 rounded text-sm transition-colors duration-200">
                        发送 (Enter)
                    </button>
                </div>
            </div>
        </div>

        <!-- Empty State -->
        <div class="flex-1 flex items-center justify-center bg-gray-50 text-gray-400 flex-col" v-else>
            <i class="fab fa-weixin text-6xl mb-4 opacity-20"></i>
            <p>选择一个联系人开始聊天</p>
        </div>
    </div>
    `
};