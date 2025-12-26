//*****************************************************************************
// Copyright 2024-2025 Intel Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//*****************************************************************************

import { useEffect } from 'react';
import { Flex, Table, Button, Space, Tag, message, Popconfirm, Spin } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import CustomBreadcrumb from '@/components/custom-breadcrumb';
import { httpRequest } from '@/utils/httpRequest';
import { useRequest } from 'ahooks';

export type PluginItem = {
  name: string;
  provider_name: string;
  services: string[];
  status: number;
  version: string;
  description: string;
};

const statusTextMap: Record<number, string> = {
  1: 'Running',
  2: 'Stopped',
  3: 'Unload',
};

const statusColorMap: Record<number, string> = {
  1: 'success',
  2: 'default',
  3: 'default',
};

export default function Plugins() {
  const {
    data: plugins,
    loading,
    refresh,
  } = useRequest<PluginItem[], []>(() => httpRequest.get('/plugin/list'), {
    manual: false,
  });

  const { runAsync: stopPlugin, loading: stopping } = useRequest((name: string) => httpRequest.post('/plugin/stop', { name }), {
    manual: true,
    onSuccess: () => {
      message.open({ content: 'Stopped successfully', type: 'success' });
      refresh();
    },
  });

  const { runAsync: deletePlugin, loading: deleting } = useRequest((name: string) => httpRequest.del('/plugin/delete', { name }), {
    manual: true,
    onSuccess: () => {
      message.open({ content: 'Deleted successfully', type: 'success' });
      refresh();
    },
  });

  const { runAsync: registerPlugin, loading: registering } = useRequest((name: string) => httpRequest.post('/plugin/register', { name }), {
    manual: true,
    onSuccess: () => {
      message.open({ content: 'Registered successfully', type: 'success' });
      refresh();
    },
  });

  useEffect(() => {
    // 初次加载由 useRequest 自动触发
  }, []);

  const columns: ColumnsType<PluginItem> = [
    {
      title: 'Plugin Name',
      dataIndex: 'name',
      key: 'name',
    },
    {
      title: 'Provider',
      dataIndex: 'provider_name',
      key: 'provider_name',
    },
    {
      title: 'Services',
      dataIndex: 'services',
      key: 'services',
      render: (services: string[]) => (
        <Space size={4} wrap>
          {services?.map((s) => (
            <Tag key={s}>{s}</Tag>
          ))}
        </Space>
      ),
    },
    {
      title: 'Version',
      dataIndex: 'version',
      key: 'version',
    },
    {
      title: 'Status',
      dataIndex: 'status',
      key: 'status',
      render: (status: number) => {
        const text = statusTextMap[status] ?? 'Unknown';
        const color = statusColorMap[status] ?? 'default';
        return <Tag color={color}>{text}</Tag>;
      },
    },
    {
      title: 'Description',
      dataIndex: 'description',
      key: 'description',
      ellipsis: true,
    },
    {
      title: 'Actions',
      key: 'actions',
      render: (_, record) => {
        const disabled = loading || stopping || deleting || registering;
        return (
          <Space>
            {(record.status === 2 || record.status === 3) && (
              <Button
                size="small"
                onClick={() => registerPlugin(record.name)}
                loading={registering}
                disabled={disabled}
              >
                Load
              </Button>
            )}
            {(record.status === 1 || record.status === 2) && (
              <Button
                size="small"
                onClick={() => stopPlugin(record.name)}
                loading={stopping}
                disabled={disabled}
              >
                Stop
              </Button>
            )}
            <Popconfirm
              title="Confirm delete?"
              onConfirm={() => deletePlugin(record.name)}
            >
              <Button
                size="small"
                danger
                loading={deleting}
                disabled={disabled}
              >
                Delete
              </Button>
            </Popconfirm>
          </Space>
        );
      },
    },
  ];

  return (
    <Flex
      vertical
      gap={24}
    >
      <CustomBreadcrumb />
      <Spin spinning={loading}>
        <Table
          rowKey="name"
          columns={columns}
          dataSource={plugins || []}
          pagination={false}
        />
      </Spin>
    </Flex>
  );
}
