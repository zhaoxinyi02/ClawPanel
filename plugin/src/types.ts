// OneBot v11 消息段类型定义

// 文本消息
export interface TextSegment {
  type: "text";
  data: { text: string };
}

// 图片消息
export interface ImageSegment {
  type: "image";
  data: {
    file: string;      // 图片文件名或URL
    url?: string;      // 图片URL
    type?: "flash";    // 闪照
    subType?: number;  // 图片子类型
    summary?: string;  // 图片摘要
  };
}

// 表情消息
export interface FaceSegment {
  type: "face";
  data: { id: string };
}

// 语音消息
export interface RecordSegment {
  type: "record";
  data: {
    file: string;
    url?: string;
    magic?: boolean;
  };
}

// 视频消息
export interface VideoSegment {
  type: "video";
  data: {
    file: string;
    url?: string;
  };
}

// @消息
export interface AtSegment {
  type: "at";
  data: { qq: string | "all" };
}

// 回复消息
export interface ReplySegment {
  type: "reply";
  data: { id: string };
}

// JSON卡片消息
export interface JsonSegment {
  type: "json";
  data: { data: string };
}

// 文件消息
export interface FileSegment {
  type: "file";
  data: {
    file: string;
    name?: string;
  };
}

// 戳一戳
export interface PokeSegment {
  type: "poke";
  data: {
    type: string;
    id: string;
  };
}

// 骰子
export interface DiceSegment {
  type: "dice";
  data: {};
}

// 猜拳
export interface RpsSegment {
  type: "rps";
  data: {};
}

// 音乐分享
export interface MusicSegment {
  type: "music";
  data: {
    type: "qq" | "163" | "custom";
    id?: string;
    url?: string;
    audio?: string;
    title?: string;
    content?: string;
    image?: string;
  };
}

// 合并转发节点
export interface NodeSegment {
  type: "node";
  data: {
    id?: string;
    user_id?: string;
    nickname?: string;
    content?: OneBotMessage;
  };
}

// 消息段联合类型
export type OneBotMessageSegment =
  | TextSegment
  | ImageSegment
  | FaceSegment
  | RecordSegment
  | VideoSegment
  | AtSegment
  | ReplySegment
  | JsonSegment
  | FileSegment
  | PokeSegment
  | DiceSegment
  | RpsSegment
  | MusicSegment
  | NodeSegment;

// 消息类型
export type OneBotMessage = OneBotMessageSegment[] | string;

// 发送者信息
export interface OneBotSender {
  user_id: number;
  nickname: string;
  card?: string;
  sex?: "male" | "female" | "unknown";
  age?: number;
  area?: string;
  level?: string;
  role?: "owner" | "admin" | "member";
  title?: string;
}

// 事件类型
export interface OneBotEvent {
  time: number;
  self_id: number;
  post_type: "message" | "notice" | "request" | "meta_event";
  
  // 消息事件
  message_type?: "private" | "group";
  sub_type?: string;
  message_id?: number;
  user_id?: number;
  group_id?: number;
  message?: OneBotMessage;
  raw_message?: string;
  font?: number;
  sender?: OneBotSender;
  
  // 元事件
  meta_event_type?: "lifecycle" | "heartbeat";
  
  // 通知事件
  notice_type?: string;
  operator_id?: number;
  
  // 请求事件
  request_type?: string;
  comment?: string;
  flag?: string;
}

// API响应
export interface OneBotResponse<T = any> {
  status: "ok" | "failed";
  retcode: number;
  data: T;
  message?: string;
  echo?: string;
}

// 好友信息
export interface FriendInfo {
  user_id: number;
  nickname: string;
  remark: string;
}

// 群信息
export interface GroupInfo {
  group_id: number;
  group_name: string;
  member_count: number;
  max_member_count: number;
}

// 群成员信息
export interface GroupMemberInfo {
  group_id: number;
  user_id: number;
  nickname: string;
  card: string;
  sex: "male" | "female" | "unknown";
  age: number;
  area: string;
  join_time: number;
  last_sent_time: number;
  level: string;
  role: "owner" | "admin" | "member";
  unfriendly: boolean;
  title: string;
  title_expire_time: number;
  card_changeable: boolean;
}

// 登录信息
export interface LoginInfo {
  user_id: number;
  nickname: string;
}

// 版本信息
export interface VersionInfo {
  app_name: string;
  app_version: string;
  protocol_version: string;
}
