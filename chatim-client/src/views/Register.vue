<template>
  <div class="register-container">
    <el-card class="register-card">
      <h2>ChatIM Register</h2>
      <el-form :model="form" label-width="80px">
        <el-form-item label="Username">
          <el-input v-model="form.username" />
        </el-form-item>
        <el-form-item label="Password">
          <el-input v-model="form.password" type="password" />
        </el-form-item>
        <el-form-item label="Nickname">
          <el-input v-model="form.nickname" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="handleRegister">Register</el-button>
          <el-button @click="$router.push('/login')">Back to Login</el-button>
        </el-form-item>
      </el-form>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { authApi } from '@/api'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'

const form = ref({
  username: '',
  password: '',
  nickname: ''
})

const router = useRouter()

const handleRegister = async () => {
  try {
    await authApi.register(form.value)
    ElMessage.success('Register successful, please login')
    router.push('/login')
  } catch (e) {
    // Error handled in interceptor
  }
}
</script>

<style scoped>
.register-container {
  display: flex;
  justify-content: center;
  align-items: center;
  height: 100vh;
  background-color: #f0f2f5;
}
.register-card {
  width: 400px;
}
</style>
