<script setup lang="ts">
import { useAuthStore } from '../stores/auth'
const auth = useAuthStore()
</script>
<template><div><div class="page-heading"><div><span class="eyebrow">RESEARCHER IDENTITY</span><h1>个人信息</h1><p>当前科研数据空间的账户信息。</p></div></div><section class="panel profile-card"><div class="profile-avatar">{{auth.user?.username?.slice(0,1).toUpperCase()}}</div><div><h2>{{auth.user?.username}}</h2><p>{{auth.user?.email}}</p><span class="status-pill">{{auth.user?.role}}</span></div></section></div></template>
