import { Component, inject, signal, ChangeDetectionStrategy } from '@angular/core';
import { ReactiveFormsModule, FormBuilder, Validators } from '@angular/forms';
import { Router, RouterLink } from '@angular/router';
import { CardModule } from 'primeng/card';
import { ButtonModule } from 'primeng/button';
import { InputTextModule } from 'primeng/inputtext';
import { TextareaModule } from 'primeng/textarea';
import { SelectModule } from 'primeng/select';
import { DatePickerModule } from 'primeng/datepicker';
import { ChipModule } from 'primeng/chip';
import { MessageModule } from 'primeng/message';
import { DividerModule } from 'primeng/divider';
import { MessageService } from 'primeng/api';
import { RegistraturaService } from '../../core/services/registratura.service';
import { TipDocument, Clasificare } from '../../core/models/document.model';

@Component({
  selector: 'app-document-create',
  imports: [
    ReactiveFormsModule, RouterLink,
    CardModule, ButtonModule, InputTextModule, TextareaModule,
    SelectModule, DatePickerModule, ChipModule, MessageModule, DividerModule
  ],
  template: `
    <div class="flex flex-col gap-6 p-6 max-w-4xl mx-auto">
      <div class="flex items-center gap-3">
        <p-button icon="pi pi-arrow-left" severity="secondary" [text]="true" routerLink="/registratura" />
        <h1 class="m-0 text-2xl font-bold">Înregistrare Document Nou</h1>
      </div>

      <form [formGroup]="form" (ngSubmit)="onSubmit()" class="flex flex-col gap-4">
        <p-card header="Date de Bază">
          <div class="flex flex-wrap gap-4">
            <div class="flex flex-col gap-2" style="flex: 1; min-width: 200px;">
              <label class="font-medium" for="tip">Tip Document <span style="color: var(--p-error-color)">*</span></label>
              <p-select
                id="tip"
                formControlName="tip"
                [options]="tipOptions"
                placeholder="Selectați tipul"
                [invalid]="isInvalid('tip')"
                styleClass="w-full"
              />
              @if (isInvalid('tip')) {
                <p-message severity="error" text="Tipul documentului este obligatoriu" />
              }
            </div>

            <div class="flex flex-col gap-2" style="flex: 1; min-width: 200px;">
              <label class="font-medium" for="clasificare">Clasificare</label>
              <p-select
                id="clasificare"
                formControlName="clasificare"
                [options]="clasificareOptions"
                placeholder="Public"
                styleClass="w-full"
              />
            </div>
          </div>
        </p-card>

        <p-card header="Conținut Document">
          <div class="flex flex-col gap-4">
            <div class="flex flex-col gap-2">
              <label class="font-medium" for="obiect">Obiect / Subiect <span style="color: var(--p-error-color)">*</span></label>
              <input
                id="obiect"
                type="text"
                pInputText
                formControlName="obiect"
                placeholder="Descriere scurtă a documentului"
                [invalid]="isInvalid('obiect')"
                class="w-full"
              />
              @if (isInvalid('obiect')) {
                <p-message severity="error" text="Obiectul documentului este obligatoriu (min. 5 caractere)" />
              }
            </div>

            <div class="flex flex-col gap-2">
              <label class="font-medium" for="continut">Conținut / Rezumat</label>
              <textarea
                id="continut"
                pTextarea
                formControlName="continut"
                rows="4"
                placeholder="Descriere detaliată..."
                class="w-full"
              ></textarea>
            </div>
          </div>
        </p-card>

        <p-card header="Date și Referințe">
          <div class="flex flex-wrap gap-4">
            <div class="flex flex-col gap-2" style="flex: 1; min-width: 180px;">
              <label class="font-medium">Data Documentului</label>
              <p-datepicker
                formControlName="data_document"
                dateFormat="dd.mm.yy"
                [showIcon]="true"
                placeholder="zi.lună.an"
                styleClass="w-full"
              />
            </div>
            <div class="flex flex-col gap-2" style="flex: 1; min-width: 180px;">
              <label class="font-medium">Termen de Răspuns</label>
              <p-datepicker
                formControlName="data_termen"
                dateFormat="dd.mm.yy"
                [showIcon]="true"
                placeholder="zi.lună.an"
                styleClass="w-full"
              />
            </div>
            <div class="flex flex-col gap-2" style="flex: 1; min-width: 200px;">
              <label class="font-medium">Nr. Document Extern</label>
              <input
                type="text"
                pInputText
                formControlName="nr_document_extern"
                placeholder="Nr. atribuit de emitent"
                class="w-full"
              />
            </div>
          </div>
        </p-card>

        <div class="flex justify-end gap-3">
          <p-button label="Anulează" severity="secondary" routerLink="/registratura" />
          <p-button
            type="submit"
            label="Înregistrează Document"
            icon="pi pi-check"
            [loading]="saving()"
            [disabled]="form.invalid"
          />
        </div>
      </form>
    </div>
  `,
  changeDetection: ChangeDetectionStrategy.OnPush
})
export class DocumentCreateComponent {
  private fb = inject(FormBuilder);
  private svc = inject(RegistraturaService);
  private router = inject(Router);
  private messageService = inject(MessageService);

  saving = signal(false);

  form = this.fb.group({
    tip: ['' as TipDocument | '', Validators.required],
    clasificare: ['PUBLIC' as Clasificare],
    obiect: ['', [Validators.required, Validators.minLength(5), Validators.maxLength(500)]],
    continut: [''],
    data_document: [null as Date | null],
    data_termen: [null as Date | null],
    nr_document_extern: [''],
  });

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
    { label: 'Raport', value: 'RAPORT' },
    { label: 'Referat', value: 'REFERAT' },
    { label: 'Adeverință', value: 'ADEVERINTA' },
    { label: 'Certificat', value: 'CERTIFICAT' },
    { label: 'Autorizație', value: 'AUTORIZATIE' },
    { label: 'Aviz', value: 'AVIZ' },
  ];

  clasificareOptions = [
    { label: 'Public', value: 'PUBLIC' },
    { label: 'Intern', value: 'INTERN' },
    { label: 'Confidențial', value: 'CONFIDENTIAL' },
    { label: 'Secret', value: 'SECRET' },
  ];

  isInvalid(field: string): boolean {
    const control = this.form.get(field);
    return !!(control?.invalid && control?.touched);
  }

  async onSubmit(): Promise<void> {
    if (this.form.invalid) {
      this.form.markAllAsTouched();
      return;
    }
    this.saving.set(true);
    try {
      const val = this.form.value;
      const doc = await this.svc.createDocument({
        registru_id: '',
        tip: val.tip as TipDocument,
        clasificare: val.clasificare as Clasificare,
        obiect: val.obiect!,
        continut: val.continut || undefined,
        data_document: val.data_document?.toISOString() || undefined,
        data_termen: val.data_termen?.toISOString() || undefined,
        nr_document_extern: val.nr_document_extern || undefined,
      });
      this.messageService.add({
        severity: 'success',
        summary: 'Document înregistrat',
        detail: `Nr. ${doc.nr_inregistrare}`
      });
      this.router.navigate(['/registratura', doc.id]);
    } catch {
      this.messageService.add({ severity: 'error', summary: 'Eroare', detail: 'Nu s-a putut înregistra documentul' });
    } finally {
      this.saving.set(false);
    }
  }
}
