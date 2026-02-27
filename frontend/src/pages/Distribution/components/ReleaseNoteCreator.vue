<script setup lang="ts">
import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import client from '../../../api/client'

const saving = ref(false)
const form = ref({
  title: '',
  version: '',
  created_by: '',
  content: '',
})

function resetForm() {
  form.value = {
    title: '',
    version: '',
    created_by: '',
    content: '',
  }
}

async function submit() {
  if (!form.value.title.trim()) {
    ElMessage.warning('请填写标题')
    return
  }
  if (!form.value.content.trim()) {
    ElMessage.warning('请填写内容')
    return
  }
  saving.value = true
  try {
    await client.post('/api/v1/release-notes', form.value)
    ElMessage.success('发布说明已创建')
    resetForm()
  } catch (err: any) {
    ElMessage.error('创建失败: ' + (err?.response?.data?.message || err.message || '未知错误'))
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="release-note-creator">
    <div class="section-header">
      <h3 class="section-title">新建发布说明</h3>
    </div>

    <el-form label-position="top" class="create-form">
      <el-form-item label="标题" required>
        <el-input v-model="form.title" placeholder="如：v2.4.1 安全补丁更新" maxlength="120" show-word-limit />
      </el-form-item>

      <el-row :gutter="12">
        <el-col :span="12">
          <el-form-item label="版本号">
            <el-input v-model="form.version" placeholder="如：v2.4.1" />
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item label="创建人">
            <el-input v-model="form.created_by" placeholder="姓名" />
          </el-form-item>
        </el-col>
      </el-row>

      <el-form-item label="发布说明内容" required>
        <el-input
          v-model="form.content"
          type="textarea"
          :rows="10"
          maxlength="10000"
          show-word-limit
          placeholder="详细描述本次发布的变更内容、修复的问题、新增功能等..."
        />
      </el-form-item>

      <div class="actions">
        <el-button @click="resetForm">重置</el-button>
        <el-button type="primary" :loading="saving" @click="submit">创建发布说明</el-button>
      </div>
    </el-form>
  </div>
</template>

<style scoped>
.release-note-creator {
  background: #fff;
  border-radius: 8px;
  padding: 20px;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.05);
}

.section-header {
  margin-bottom: 16px;
}

.section-title {
  font-size: 18px;
  font-weight: 600;
  margin: 0;
  color: #303133;
}

.actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}
</style>
