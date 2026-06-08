export interface Country { id: number; code: string; name: string; phone_code: string }
export interface State { id: number; country_id: number; code: string; name: string }
export interface City { id: number; state_id: number; name: string }
export interface LocalityType { id: number; code: string; name: string }
export interface Bank { id: number; code: string; name: string }
export interface TaxRegime { id: number; code: string; name: string; persona_fisica: boolean; persona_moral: boolean }
export interface PaymentForm { id: number; code: string; name: string }
export interface PaymentCondition { id: number; code: string; name: string; days: number }
export interface WorkflowStatus { id: number; code: string; name: string; role_id: number }
export interface ComplaintType { id: number; code: string; name: string; description?: string }
export interface CatalogService { id: number; code: string; name: string; description?: string; price: number; is_active: boolean }
export interface SubscriptionPlan { id: number; code: string; name: string; amount: number }
export interface DatePeriodicity { id: number; code: string; name: string }
export interface HrAbsenceType { id: number; code: string; name: string; requires_justification: boolean }
export interface JobCategory { id: number; code: string; name: string }
export interface JobType { id: number; code: string; name: string }

export interface ProblemDetail { code: string; message: string; errors?: Record<string, string> }

export interface SimpleCatalogItem { id: number; code: string; name: string }
