export type DocumentStatus =
  | 'INREGISTRAT' | 'ALOCAT_COMPARTIMENT' | 'IN_LUCRU'
  | 'FLUX_APROBARE' | 'FINALIZAT' | 'ARHIVAT' | 'ANULAT';

export type TipDocument =
  | 'INTRARE' | 'IESIRE' | 'INTERN' | 'PETITIE' | 'CONTRACT'
  | 'DECIZIE' | 'HOTARARE' | 'DISPOZITIE' | 'ADRESA' | 'NOTIFICARE'
  | 'RAPORT' | 'REFERAT' | 'ADEVERINTA' | 'CERTIFICAT' | 'AUTORIZATIE' | 'AVIZ';

export type Clasificare = 'PUBLIC' | 'INTERN' | 'CONFIDENTIAL' | 'SECRET';

export interface Document {
  id: string;
  nr_inregistrare: string;
  registru_id: string;
  institution_id: string;
  tip: TipDocument;
  status: DocumentStatus;
  clasificare: Clasificare;
  emitent_id?: string;
  destinatar_id?: string;
  obiect: string;
  continut?: string;
  cuvinte_cheie?: string[];
  data_inregistrare: string;
  data_document?: string;
  data_termen?: string;
  data_finalizare?: string;
  termen_pastrare_ani: number;
  archive_status: string;
  rejection_count: number;
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface CreateDocumentDTO {
  registru_id: string;
  tip: TipDocument;
  clasificare: Clasificare;
  emitent_id?: string;
  destinatar_id?: string;
  obiect: string;
  continut?: string;
  cuvinte_cheie?: string[];
  nr_file?: number;
  data_document?: string;
  data_termen?: string;
  nr_document_extern?: string;
}

export interface PaginatedDocuments {
  data: Document[];
  total: number;
  page: number;
  limit: number;
}
