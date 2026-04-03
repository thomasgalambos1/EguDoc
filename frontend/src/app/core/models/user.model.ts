export interface User {
  id: string;
  subject: string;
  email: string;
  phone?: string;
  prenume?: string;
  nume?: string;
  active: boolean;
  created_at: string;
}
