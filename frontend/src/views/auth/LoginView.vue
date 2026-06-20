<template>
  <AuthLayout>
    <LoginForm consume-expired-session />
  </AuthLayout>
</template>

<script setup lang="ts">
import { AuthLayout } from '@/components/layout'
import LoginForm from '@/components/auth/LoginForm.vue'
</script>
