import type { z } from 'zod'
import type {
  companyInactiveDaysSchema,
  errorEnvelopeSchema,
  inactiveDayFormSchema,
  inactiveDaySchema,
  inactiveDaysThresholdResponseSchema,
  inactiveDaysThresholdSchema,
  personalEvaluationFormSchema,
  personalEvaluationSchema,
  serviceEvaluationFormSchema,
  serviceEvaluationSchema,
  systemSettingFormSchema,
  systemSettingSchema,
} from '../schemas/validation'

export type ErrorEnvelope = z.infer<typeof errorEnvelopeSchema>
export type ServiceEvaluation = z.infer<typeof serviceEvaluationSchema>
export type ServiceEvaluationForm = z.infer<typeof serviceEvaluationFormSchema>
export type PersonalEvaluation = z.infer<typeof personalEvaluationSchema>
export type PersonalEvaluationForm = z.infer<typeof personalEvaluationFormSchema>
export type InactiveDay = z.infer<typeof inactiveDaySchema>
export type InactiveDayForm = z.infer<typeof inactiveDayFormSchema>
export type CompanyInactiveDays = z.infer<typeof companyInactiveDaysSchema>
export type InactiveDaysThreshold = z.infer<typeof inactiveDaysThresholdSchema>
export type InactiveDaysThresholdResponse = z.infer<typeof inactiveDaysThresholdResponseSchema>
export type SystemSetting = z.infer<typeof systemSettingSchema>
export type SystemSettingForm = z.infer<typeof systemSettingFormSchema>
