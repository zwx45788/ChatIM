# 友谊服务测试脚本 (PowerShell)

# 配置
$GRPC_HOST = "localhost:50053"
$JWT_TOKEN = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoidXNlcl8xIiwicHlwaS1kdWNrLWNsaWVudC1pcCI6Ijw0QS1jb21wYWN0LT5QYVczZ0FkQXdIRVJBZWdRZWdRZWdRZWdRZWdRZWdRZWdRZWdRZWdRZWdRZWdRZWdRZWdRZWdRZWdRZWdRZWdRZWdRZWdRIn0.kKaR77KeXVBHBXzBzQP8K7Y5ZjKdZvVW8LqQ1g5JcKE"

Write-Host "===========================================" -ForegroundColor Cyan
Write-Host "友谊服务 gRPC API 测试" -ForegroundColor Cyan
Write-Host "===========================================" -ForegroundColor Cyan
Write-Host ""

# 测试函数
function Test-Endpoint {
    param(
        [string]$Method,
        [string]$Request,
        [string]$Description
    )
    
    Write-Host "[测试] $Description" -ForegroundColor Yellow
    Write-Host "方法: $Method"
    Write-Host "请求: $Request"
    
    try {
        $response = grpcurl -plaintext `
            -d $Request `
            -H "authorization: Bearer $JWT_TOKEN" `
            $GRPC_HOST `
            ChatIM.friendship.FriendshipService/$Method 2>&1
        
        Write-Host "响应: $response"
        Write-Host ""
        
        if ($response -match '"code"\s*:\s*0') {
            Write-Host "✓ 测试通过" -ForegroundColor Green
        } else {
            Write-Host "✗ 测试失败" -ForegroundColor Red
        }
    }
    catch {
        Write-Host "✗ 错误: $_" -ForegroundColor Red
    }
    
    Write-Host "---" -ForegroundColor Gray
    Write-Host ""
}

# 1. 发送好友请求
Test-Endpoint `
    "SendFriendRequest" `
    '{"to_user_id":"user_2","message":"你好，可以加好友吗？"}' `
    "发送好友请求"

# 2. 获取待处理好友请求
Test-Endpoint `
    "GetFriendRequests" `
    '{"status":0,"limit":20,"offset":0}' `
    "获取待处理好友请求"

# 3. 获取已接受的好友请求
Test-Endpoint `
    "GetFriendRequests" `
    '{"status":1,"limit":20,"offset":0}' `
    "获取已接受的好友请求"

# 4. 获取好友列表
Test-Endpoint `
    "GetFriends" `
    '{"limit":20,"offset":0}' `
    "获取好友列表"

# 5. 申请加入群组
Test-Endpoint `
    "SendGroupJoinRequest" `
    '{"group_id":"group_1","message":"申请加入群组"}' `
    "申请加入群组"

# 6. 获取群申请列表
Test-Endpoint `
    "GetGroupJoinRequests" `
    '{"group_id":"group_1","status":0,"limit":20,"offset":0}' `
    "获取群申请列表"

Write-Host "=========================================" -ForegroundColor Green
Write-Host "所有测试完成！" -ForegroundColor Green
Write-Host "==========================================" -ForegroundColor Green
