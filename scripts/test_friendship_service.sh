#!/bin/bash
# 友谊服务测试脚本

set -e

# 配置
GRPC_HOST="localhost:50053"
JWT_TOKEN="${JWT_TOKEN:-$(echo 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoidXNlcl8xIiwicHlwaS1kdWNrLWNsaWVudC1pcCI6Ijw0QS1jb21wYWN0LT5QYVczZ0FkQXdIRVJBZWdRZWdRZWdRZWdRZWdRZWdRZWdRZWdRZWdRZWdRZWdRZWdRZWdRZWdRZWdRZWdRZWdRZWdRZWdRIn0.kKaR77KeXVBHBXzBzQP8K7Y5ZjKdZvVW8LqQ1g5JcKE')}"

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "=========================================="
echo "友谊服务 gRPC API 测试"
echo "=========================================="
echo ""

# 测试函数
test_endpoint() {
    local method=$1
    local request=$2
    local description=$3
    
    echo -e "${YELLOW}[测试] $description${NC}"
    echo "方法: $method"
    echo "请求: $request"
    
    response=$(grpcurl -plaintext \
        -d "$request" \
        -H "authorization: Bearer $JWT_TOKEN" \
        $GRPC_HOST \
        ChatIM.friendship.FriendshipService/$method 2>&1 || true)
    
    echo "响应: $response"
    echo ""
    
    if echo "$response" | grep -q "code.*0"; then
        echo -e "${GREEN}✓ 测试通过${NC}"
    else
        echo -e "${RED}✗ 测试失败${NC}"
    fi
    echo "---"
    echo ""
}

# 1. 发送好友请求
test_endpoint \
    "SendFriendRequest" \
    '{"to_user_id":"user_2","message":"你好，可以加好友吗？"}' \
    "发送好友请求"

# 2. 获取待处理好友请求
test_endpoint \
    "GetFriendRequests" \
    '{"status":0,"limit":20,"offset":0}' \
    "获取待处理好友请求"

# 3. 获取已接受的好友请求
test_endpoint \
    "GetFriendRequests" \
    '{"status":1,"limit":20,"offset":0}' \
    "获取已接受的好友请求"

# 4. 获取好友列表
test_endpoint \
    "GetFriends" \
    '{"limit":20,"offset":0}' \
    "获取好友列表"

# 5. 申请加入群组
test_endpoint \
    "SendGroupJoinRequest" \
    '{"group_id":"group_1","message":"申请加入群组"}' \
    "申请加入群组"

# 6. 获取群申请列表
test_endpoint \
    "GetGroupJoinRequests" \
    '{"group_id":"group_1","status":0,"limit":20,"offset":0}' \
    "获取群申请列表"

echo -e "${GREEN}=========================================="
echo "所有测试完成！"
echo "==========================================${NC}"
