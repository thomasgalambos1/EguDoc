import { Component, OnInit, inject, signal, ChangeDetectionStrategy } from '@angular/core';
import { DatePipe } from '@angular/common';
import { ActivatedRoute, RouterLink } from '@angular/router';
import { ButtonModule } from 'primeng/button';
import { CardModule } from 'primeng/card';
import { TagModule } from 'primeng/tag';
import { SkeletonModule } from 'primeng/skeleton';
import { MessageModule } from 'primeng/message';
import { RegistraturaService } from '../../core/services/registratura.service';
import { Document, DocumentStatus } from '../../core/models/document.model';

@Component({
  selector: 'app-document-detail',
  imports: [DatePipe, RouterLink, ButtonModule, CardModule, TagModule, SkeletonModule, MessageModule],
  template: `
    <div class="flex flex-col gap-4 p-6 max-w-4xl mx-auto">
      <div class="flex items-center gap-3">
        <p-button icon="pi pi-arrow-left" severity="secondary" [text]="true" routerLink="/registratura" />
        @if (loading()) {
          <p-skeleton width="16rem" height="2rem" />
        } @else if (document()) {
          <h1 class="m-0 text-2xl font-bold">{{ document()!.nr_inregistrare }}</h1>
        }
      </div>

      @if (error()) {
        <p-message severity="error" [text]="error()!" />
      }

      @if (loading()) {
        <p-card>
          <div class="flex flex-col gap-3">
            @for (_ of [1,2,3,4]; track $index) {
              <p-skeleton height="1.5rem" />
            }
          </div>
        </p-card>
      } @else if (document(); as doc) {
        <p-card header="Informații Document">
          <div class="flex flex-col gap-4">
            <div class="flex flex-wrap gap-6">
              <div class="flex flex-col gap-1">
                <span class="text-sm font-medium" style="color: var(--p-text-muted-color)">Tip</span>
                <p-tag [value]="doc.tip" severity="secondary" />
              </div>
              <div class="flex flex-col gap-1">
                <span class="text-sm font-medium" style="color: var(--p-text-muted-color)">Status</span>
                <p-tag [value]="statusLabel(doc.status)" [severity]="statusSeverity(doc.status)" />
              </div>
              <div class="flex flex-col gap-1">
                <span class="text-sm font-medium" style="color: var(--p-text-muted-color)">Clasificare</span>
                <span class="font-medium">{{ doc.clasificare }}</span>
              </div>
              <div class="flex flex-col gap-1">
                <span class="text-sm font-medium" style="color: var(--p-text-muted-color)">Data Înregistrare</span>
                <span>{{ doc.data_inregistrare | date:'dd.MM.yyyy HH:mm' }}</span>
              </div>
              @if (doc.data_termen) {
                <div class="flex flex-col gap-1">
                  <span class="text-sm font-medium" style="color: var(--p-text-muted-color)">Termen Răspuns</span>
                  <span>{{ doc.data_termen | date:'dd.MM.yyyy' }}</span>
                </div>
              }
            </div>

            <div class="flex flex-col gap-1">
              <span class="text-sm font-medium" style="color: var(--p-text-muted-color)">Obiect</span>
              <p class="m-0 font-medium">{{ doc.obiect }}</p>
            </div>

            @if (doc.continut) {
              <div class="flex flex-col gap-1">
                <span class="text-sm font-medium" style="color: var(--p-text-muted-color)">Conținut</span>
                <p class="m-0">{{ doc.continut }}</p>
              </div>
            }

            @if (doc.cuvinte_cheie?.length) {
              <div class="flex flex-col gap-2">
                <span class="text-sm font-medium" style="color: var(--p-text-muted-color)">Cuvinte Cheie</span>
                <div class="flex flex-wrap gap-2">
                  @for (kw of doc.cuvinte_cheie!; track kw) {
                    <p-tag [value]="kw" severity="info" />
                  }
                </div>
              </div>
            }
          </div>
        </p-card>
      }
    </div>
  `,
  changeDetection: ChangeDetectionStrategy.OnPush
})
export class DocumentDetailComponent implements OnInit {
  private route = inject(ActivatedRoute);
  private svc = inject(RegistraturaService);

  loading = signal(true);
  document = signal<Document | null>(null);
  error = signal<string | null>(null);

  ngOnInit(): void {
    const id = this.route.snapshot.paramMap.get('id');
    if (!id) {
      this.error.set('ID document lipsă');
      this.loading.set(false);
      return;
    }
    this.loadDocument(id);
  }

  private async loadDocument(id: string): Promise<void> {
    try {
      const doc = await this.svc.getDocument(id);
      this.document.set(doc);
    } catch {
      this.error.set('Documentul nu a putut fi încărcat. Verificați conexiunea și încercați din nou.');
    } finally {
      this.loading.set(false);
    }
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
