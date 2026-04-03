import { Injectable, inject } from '@angular/core';
import { HttpClient, HttpParams } from '@angular/common/http';
import { firstValueFrom } from 'rxjs';
import { environment } from '../../../environments/environment';
import { Document, CreateDocumentDTO, PaginatedDocuments } from '../models/document.model';

export interface ListDocumentsParams {
  page?: number;
  limit?: number;
  status?: string;
  tip?: string;
  registru_id?: string;
  search?: string;
  data_de?: string;
  data_pana?: string;
}

@Injectable({ providedIn: 'root' })
export class RegistraturaService {
  private http = inject(HttpClient);
  private base = `${environment.apiUrl}/api/documents`;

  async getDocuments(params: ListDocumentsParams = {}): Promise<PaginatedDocuments> {
    let httpParams = new HttpParams();
    if (params.page) httpParams = httpParams.set('page', params.page);
    if (params.limit) httpParams = httpParams.set('limit', params.limit);
    if (params.status) httpParams = httpParams.set('status', params.status);
    if (params.tip) httpParams = httpParams.set('tip', params.tip);
    if (params.registru_id) httpParams = httpParams.set('registru_id', params.registru_id);
    if (params.search) httpParams = httpParams.set('search', params.search);
    if (params.data_de) httpParams = httpParams.set('data_de', params.data_de);
    if (params.data_pana) httpParams = httpParams.set('data_pana', params.data_pana);
    return firstValueFrom(this.http.get<PaginatedDocuments>(this.base, { params: httpParams }));
  }

  async getDocument(id: string): Promise<Document> {
    return firstValueFrom(this.http.get<Document>(`${this.base}/${id}`));
  }

  async createDocument(dto: CreateDocumentDTO): Promise<Document> {
    return firstValueFrom(this.http.post<Document>(this.base, dto));
  }

  async updateDocument(id: string, dto: Partial<CreateDocumentDTO>): Promise<Document> {
    return firstValueFrom(this.http.patch<Document>(`${this.base}/${id}`, dto));
  }

  async performWorkflowAction(documentId: string, action: string, payload: Record<string, unknown>): Promise<unknown> {
    return firstValueFrom(
      this.http.post(`${environment.apiUrl}/api/workflows/${documentId}/actions`, { action, ...payload })
    );
  }

  async uploadAttachment(documentId: string, file: File, description?: string): Promise<unknown> {
    const formData = new FormData();
    formData.append('file', file);
    if (description) formData.append('description', description);
    return firstValueFrom(this.http.post(`${this.base}/${documentId}/attachments`, formData));
  }
}
