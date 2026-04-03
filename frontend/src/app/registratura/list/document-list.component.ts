import { Component, OnInit, inject, signal, ChangeDetectionStrategy } from '@angular/core';
import { DatePipe, SlicePipe } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { RouterLink } from '@angular/router';
import { TableModule } from 'primeng/table';
import { ButtonModule } from 'primeng/button';
import { TagModule } from 'primeng/tag';
import { InputTextModule } from 'primeng/inputtext';
import { SelectModule } from 'primeng/select';
import { CardModule } from 'primeng/card';
import { ToolbarModule } from 'primeng/toolbar';
import { TooltipModule } from 'primeng/tooltip';
import { IconFieldModule } from 'primeng/iconfield';
import { InputIconModule } from 'primeng/inputicon';
import { Document, DocumentStatus } from '../../core/models/document.model';
import { RegistraturaService, ListDocumentsParams } from '../../core/services/registratura.service';

@Component({
  selector: 'app-document-list',
  imports: [
    DatePipe, SlicePipe, FormsModule, RouterLink,
    TableModule, ButtonModule, TagModule, InputTextModule, SelectModule,
    CardModule, ToolbarModule, TooltipModule, IconFieldModule, InputIconModule
  ],
  template: `
    <div class="flex flex-col gap-4 p-6">
      <div class="flex items-center justify-between">
        <h1 class="m-0 text-2xl font-bold">Registratură</h1>
        <p-button label="Document Nou" icon="pi pi-plus" routerLink="nou" />
      </div>

      <p-card>
        <div class="flex flex-wrap gap-3 items-end">
          <div class="flex flex-col gap-1">
            <label class="text-sm font-medium">Caută</label>
            <p-iconField iconPosition="left">
              <p-inputIcon styleClass="pi pi-search" />
              <input
                pInputText
                type="text"
                placeholder="Obiect, nr. înregistrare..."
                [(ngModel)]="searchQuery"
                (ngModelChange)="onSearchChange()"
                style="width: 280px;"
              />
            </p-iconField>
          </div>

          <div class="flex flex-col gap-1">
            <label class="text-sm font-medium">Status</label>
            <p-select
              [options]="statusOptions"
              [(ngModel)]="selectedStatus"
              (ngModelChange)="loadDocuments()"
              placeholder="Toate statusurile"
              [showClear]="true"
              style="width: 180px;"
            />
          </div>

          <div class="flex flex-col gap-1">
            <label class="text-sm font-medium">Tip Document</label>
            <p-select
              [options]="tipOptions"
              [(ngModel)]="selectedTip"
              (ngModelChange)="loadDocuments()"
              placeholder="Toate tipurile"
              [showClear]="true"
              style="width: 180px;"
            />
          </div>

          <p-button
            icon="pi pi-filter-slash"
            severity="secondary"
            label="Resetează"
            (onClick)="resetFilters()"
          />
        </div>
      </p-card>

      <p-table
        [value]="documents()"
        [loading]="loading()"
        [lazy]="true"
        [paginator]="true"
        [rows]="pageSize"
        [totalRecords]="totalRecords()"
        [rowsPerPageOptions]="[10, 25, 50]"
        (onPage)="onPageChange($event)"
        styleClass="p-datatable-gridlines p-datatable-striped"
        [scrollable]="true"
        scrollHeight="calc(100vh - 380px)"
      >
        <ng-template #header>
          <tr>
            <th style="width: 160px;">Nr. Înregistrare</th>
            <th style="width: 120px;">Tip</th>
            <th>Obiect</th>
            <th style="width: 130px;">Status</th>
            <th style="width: 110px;">Data</th>
            <th style="width: 110px;">Termen</th>
            <th style="width: 80px;"></th>
          </tr>
        </ng-template>

        <ng-template #body let-doc>
          <tr>
            <td>
              <code class="font-mono text-sm">{{ doc.nr_inregistrare }}</code>
            </td>
            <td>
              <p-tag [value]="doc.tip" severity="secondary" />
            </td>
            <td>
              <span [pTooltip]="doc.obiect" tooltipPosition="top">
                {{ doc.obiect | slice:0:60 }}{{ doc.obiect.length > 60 ? '...' : '' }}
              </span>
            </td>
            <td>
              <p-tag
                [value]="statusLabel(doc.status)"
                [severity]="statusSeverity(doc.status)"
              />
            </td>
            <td>{{ doc.data_inregistrare | date:'dd.MM.yyyy' }}</td>
            <td>
              @if (doc.data_termen) {
                {{ doc.data_termen | date:'dd.MM.yyyy' }}
              }
            </td>
            <td>
              <p-button
                icon="pi pi-eye"
                severity="secondary"
                [text]="true"
                [rounded]="true"
                [routerLink]="['/registratura', doc.id]"
                pTooltip="Vizualizează"
              />
            </td>
          </tr>
        </ng-template>

        <ng-template #emptymessage>
          <tr>
            <td colspan="7">
              <div class="flex flex-col items-center gap-3 p-8">
                <i class="pi pi-inbox text-5xl" style="color: var(--p-text-muted-color)"></i>
                <span style="color: var(--p-text-muted-color)">Nu există documente cu filtrele aplicate</span>
                <p-button label="Înregistrează Document" routerLink="nou" />
              </div>
            </td>
          </tr>
        </ng-template>
      </p-table>
    </div>
  `,
  changeDetection: ChangeDetectionStrategy.OnPush
})
export class DocumentListComponent implements OnInit {
  private svc = inject(RegistraturaService);

  documents = signal<Document[]>([]);
  loading = signal(true);
  totalRecords = signal(0);

  page = 1;
  pageSize = 25;
  searchQuery = '';
  selectedStatus = '';
  selectedTip = '';
  private searchTimeout: ReturnType<typeof setTimeout> | null = null;

  statusOptions = [
    { label: 'Înregistrat', value: 'INREGISTRAT' },
    { label: 'Alocat Compartiment', value: 'ALOCAT_COMPARTIMENT' },
    { label: 'În Lucru', value: 'IN_LUCRU' },
    { label: 'Flux Aprobare', value: 'FLUX_APROBARE' },
    { label: 'Finalizat', value: 'FINALIZAT' },
    { label: 'Arhivat', value: 'ARHIVAT' },
    { label: 'Anulat', value: 'ANULAT' },
  ];

  tipOptions = [
    { label: 'Intrare', value: 'INTRARE' },
    { label: 'Ieșire', value: 'IESIRE' },
    { label: 'Intern', value: 'INTERN' },
    { label: 'Petiție', value: 'PETITIE' },
    { label: 'Contract', value: 'CONTRACT' },
    { label: 'Decizie', value: 'DECIZIE' },
    { label: 'Hotărâre', value: 'HOTARARE' },
    { label: 'Dispoziție', value: 'DISPOZITIE' },
    { label: 'Adresă', value: 'ADRESA' },
  ];

  ngOnInit(): void {
    this.loadDocuments();
  }

  async loadDocuments(): Promise<void> {
    this.loading.set(true);
    try {
      const params: ListDocumentsParams = { page: this.page, limit: this.pageSize };
      if (this.selectedStatus) params.status = this.selectedStatus;
      if (this.selectedTip) params.tip = this.selectedTip;
      if (this.searchQuery) params.search = this.searchQuery;

      const result = await this.svc.getDocuments(params);
      this.documents.set(result.data);
      this.totalRecords.set(result.total);
    } catch {
      // API not available yet during development
    } finally {
      this.loading.set(false);
    }
  }

  onSearchChange(): void {
    if (this.searchTimeout) clearTimeout(this.searchTimeout);
    this.searchTimeout = setTimeout(() => this.loadDocuments(), 400);
  }

  onPageChange(event: { first: number; rows: number }): void {
    this.page = Math.floor(event.first / event.rows) + 1;
    this.pageSize = event.rows;
    this.loadDocuments();
  }

  resetFilters(): void {
    this.searchQuery = '';
    this.selectedStatus = '';
    this.selectedTip = '';
    this.page = 1;
    this.loadDocuments();
  }

  statusLabel(status: DocumentStatus): string {
    const labels: Record<DocumentStatus, string> = {
      INREGISTRAT: 'Înregistrat', ALOCAT_COMPARTIMENT: 'Alocat',
      IN_LUCRU: 'În Lucru', FLUX_APROBARE: 'Aprobare',
      FINALIZAT: 'Finalizat', ARHIVAT: 'Arhivat', ANULAT: 'Anulat',
    };
    return labels[status];
  }

  statusSeverity(status: DocumentStatus): 'success' | 'info' | 'warn' | 'danger' | 'secondary' {
    const map: Record<DocumentStatus, 'success' | 'info' | 'warn' | 'danger' | 'secondary'> = {
      INREGISTRAT: 'info', ALOCAT_COMPARTIMENT: 'warn', IN_LUCRU: 'warn',
      FLUX_APROBARE: 'warn', FINALIZAT: 'success', ARHIVAT: 'secondary', ANULAT: 'danger',
    };
    return map[status];
  }
}
