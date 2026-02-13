<script setup lang="ts">
import { ref, reactive } from 'vue'
import { ElMessage } from 'element-plus'
import type { DocumentFeedback } from '../pages/Documents/types'

defineProps<{
  visible: boolean
  docSlug?: string
}>()

const emit = defineEmits<{
  'update:visible': [value: boolean]
  'submit': [feedback: DocumentFeedback]
}>()

const formRef = ref()
const form = reactive<DocumentFeedback>({
  type: 'content',
  description: '',
  email: '',
})

const rules = {
  description: [
    { required: true, message: '请填写详细描述', trigger: 'blur' },
    { min: 10, message: '描述至少需要 10 个字符', trigger: 'blur' },
  ],
  email: [
    { type: 'email' as const, message: '请输入有效的邮箱地址', trigger: 'blur' },
  ],
}

function handleClose() {
  emit('update:visible', false)
}

async function handleSubmit() {
  if (!formRef.value) return
  try {
    await formRef.value.validate()
    emit('submit', { ...form })
    ElMessage.success('感谢您的反馈！我们会尽快处理。')
    // 重置表单
    form.type = 'content'
    form.description = ''
    form.email = ''
    emit('update:visible', false)
  } catch {
    // 验证失败，不做处理
  }
}
</script>

<template>
  <el-dialog
    :model-value="visible"
    title="文档反馈"
    width="500px"
    destroy-on-close
    @update:model-value="emit('update:visible', $event)"
  >
    <el-form ref="formRef" :model="form" :rules="rules" label-position="top">
      <el-form-item label="反馈类型" prop="type">
        <el-radio-group v-model="form.type">
          <el-radio-button value="content">内容错误</el-radio-button>
          <el-radio-button value="missing">缺少信息</el-radio-button>
          <el-radio-button value="other">其他建议</el-radio-button>
        </el-radio-group>
      </el-form-item>
      <el-form-item label="详细描述" prop="description">
        <el-input
          v-model="form.description"
          type="textarea"
          :rows="4"
          placeholder="请描述您遇到的问题或建议..."
        />
      </el-form-item>
      <el-form-item label="联系邮箱（可选）" prop="email">
        <el-input v-model="form.email" placeholder="用于接收处理结果" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="handleClose">取消</el-button>
      <el-button type="primary" @click="handleSubmit">提交反馈</el-button>
    </template>
  </el-dialog>
</template>
