import type { DeliveryAddress } from '@/types';

export type ZipLookup =
  | { status: 'found'; address: Pick<DeliveryAddress, 'street' | 'district' | 'city' | 'state'> }
  | { status: 'not-found' }
  | { status: 'error' };

/**
 * Consulta o CEP na ViaCEP para adiantar o preenchimento. É só uma ajuda: se
 * o serviço estiver fora, a pessoa digita o endereço normalmente.
 */
export async function lookupZipCode(zip: string, signal?: AbortSignal): Promise<ZipLookup> {
  const digits = zip.replace(/\D/g, '');
  if (digits.length !== 8) return { status: 'not-found' };
  try {
    const res = await fetch(`https://viacep.com.br/ws/${digits}/json/`, { signal });
    if (!res.ok) return { status: res.status === 400 ? 'not-found' : 'error' };
    const data = (await res.json()) as {
      erro?: boolean | string;
      logradouro?: string;
      bairro?: string;
      localidade?: string;
      uf?: string;
    };
    if (data.erro) return { status: 'not-found' };
    return {
      status: 'found',
      address: {
        street: data.logradouro ?? '',
        district: data.bairro ?? '',
        city: data.localidade ?? '',
        state: data.uf ?? '',
      },
    };
  } catch {
    return { status: 'error' };
  }
}
