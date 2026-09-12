export interface UploadApiResult {
  code: number;
  msg: string;
  data: string;
  file?: {
    id: number;
    filename: string;
    size: number;
    content_type: string;
    private: boolean;
    max_size: number;
    download_path: string;
  };
}
