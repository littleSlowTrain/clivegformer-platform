<script setup lang="ts">
import { computed } from 'vue'
import { Collection, DataAnalysis, Files, House, UploadFilled, User } from '@element-plus/icons-vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const logout = () => { auth.logout(); router.push('/login') }
const title = computed(() => ({
  '/dashboard': '科研数据工作台',
  '/team-files': '课题组数据空间',
  '/files': '我的数据空间',
  '/uploads': '上传任务',
  '/analysis': '智能分析',
  '/profile': '个人信息',
}[route.path] || '数据空间'))
</script>

<template>
  <div class="shell">
    <aside class="sidebar">
      <div class="brand"><div class="brand-mark">CV</div><div><strong>CliVegFormer</strong><span>遥感科研数据平台</span></div></div>
      <el-menu router :default-active="route.path">
        <el-menu-item index="/dashboard"><el-icon><House /></el-icon><span>数据总览</span></el-menu-item>
        <el-menu-item index="/team-files"><el-icon><Collection /></el-icon><span>课题组数据</span></el-menu-item>
        <el-menu-item index="/files"><el-icon><Files /></el-icon><span>我的数据</span></el-menu-item>
        <el-menu-item index="/uploads"><el-icon><UploadFilled /></el-icon><span>上传任务</span></el-menu-item>
        <el-menu-item index="/analysis"><el-icon><DataAnalysis /></el-icon><span>智能分析</span></el-menu-item>
        <el-menu-item index="/profile"><el-icon><User /></el-icon><span>个人信息</span></el-menu-item>
      </el-menu>
      <div class="storage-note"><span class="status-dot"></span><div><b>Ceph RGW</b><small>192.168.10.130</small></div></div>
    </aside>
    <main class="main">
      <header class="topbar">
        <div><span class="eyebrow">REMOTE SENSING RESEARCH CLOUD</span><h2>{{ title }}</h2></div>
        <el-dropdown><button class="user-chip"><span>{{ auth.user?.username?.slice(0, 1).toUpperCase() }}</span>{{ auth.user?.username }}</button><template #dropdown><el-dropdown-menu><el-dropdown-item @click="logout">退出登录</el-dropdown-item></el-dropdown-menu></template></el-dropdown>
      </header>
      <section class="content"><router-view /></section>
    </main>
  </div>
</template>
